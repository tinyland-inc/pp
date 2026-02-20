package widgets

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"gitlab.com/tinyland/lab/prompt-pulse/pkg/app"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/components"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/waifu"
)

// ImageRenderer is the interface for rendering an image file to terminal
// escape sequences. It mirrors the method on pkg/image.Renderer so the
// widget can be tested with a mock.
type ImageRenderer interface {
	RenderFile(path string, width, height int) (string, error)
}

// WaifuRefreshMsg is sent when the user presses 'r' to request a new image.
type WaifuRefreshMsg struct{}

// WaifuWidget displays a waifu image in the dashboard, implementing the
// app.Widget interface. It receives rendered image data through Update and
// caches the output to avoid redundant rendering on identical dimensions.
type WaifuWidget struct {
	id          string
	session     *waifu.Session
	sessionMgr  *waifu.SessionManager
	renderer    ImageRenderer
	rendered    string // cached rendered output
	lastWidth   int
	lastHeight  int
	overlayText string // character name / source
	showInfo    bool   // whether the info overlay is active
	loading     bool
	err         error

	// Sequential navigation state
	imageList  []string // sorted list of image paths in the directory
	imageIndex int      // current position in imageList (-1 = unknown)
	imageDir   string   // directory containing images
}

// NewWaifuWidget creates a WaifuWidget bound to the given session manager
// and image renderer. It obtains or creates a session immediately and marks
// itself as loading until the first render completes. The imageDir parameter
// enables sequential navigation (next/prev) through images in that directory.
func NewWaifuWidget(sessionMgr *waifu.SessionManager, renderer ImageRenderer, imageDir string) *WaifuWidget {
	w := &WaifuWidget{
		id:         "waifu",
		sessionMgr: sessionMgr,
		renderer:   renderer,
		loading:    true,
		imageDir:   imageDir,
		imageIndex: -1,
	}

	// Load the image list for sequential navigation.
	if imageDir != "" {
		if images, err := waifu.ListImages(imageDir); err == nil && len(images) > 0 {
			sort.Strings(images)
			w.imageList = images
		}
	}

	// Attempt to get or create a session. If the image directory is
	// missing or empty, we capture the error and display it later.
	session, err := sessionMgr.GetOrCreate()
	if err != nil {
		w.err = fmt.Errorf("waifu init: %w (add images to cache dir)", err)
		w.loading = false
	} else {
		w.session = session
		w.overlayText = formatImageName(session.ImagePath)
		w.imageIndex = w.findImageIndex(session.ImagePath)
	}

	return w
}

// ID returns "waifu".
func (w *WaifuWidget) ID() string {
	return w.id
}

// Title returns "Waifu" or the character name with an index indicator
// (e.g. "sakura bloom [3/200]") if a session is active and the image
// list has been loaded.
func (w *WaifuWidget) Title() string {
	name := w.overlayText
	if name == "" {
		name = "Waifu"
	}
	if w.imageIndex >= 0 && len(w.imageList) > 0 {
		return fmt.Sprintf("%s [%d/%d]", name, w.imageIndex+1, len(w.imageList))
	}
	return name
}

// Update handles messages directed at this widget. It processes
// DataUpdateEvent from the waifu collector and window resize events.
func (w *WaifuWidget) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case app.DataUpdateEvent:
		if msg.Source != "waifu" {
			return nil
		}
		if msg.Err != nil {
			w.err = msg.Err
			w.loading = false
			return nil
		}
		// Data is expected to be a rendered string or a *waifu.Session.
		switch data := msg.Data.(type) {
		case string:
			w.rendered = data
			w.loading = false
			w.err = nil
		case *waifu.Session:
			w.session = data
			w.overlayText = formatImageName(data.ImagePath)
			// Invalidate cache so the next View call re-renders.
			w.lastWidth = 0
			w.lastHeight = 0
			w.loading = true
			w.err = nil
		}
		return nil

	case WaifuRefreshMsg:
		return w.refresh()

	case tea.WindowSizeMsg:
		// Invalidate cached render on resize.
		w.lastWidth = 0
		w.lastHeight = 0
		return nil
	}

	return nil
}

// View renders the waifu image to fill the given area. The returned string
// fits within width x height cells.
func (w *WaifuWidget) View(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	// Error state: show the error in a box.
	if w.err != nil {
		return w.renderError(width, height)
	}

	// Loading state: show a centered loading indicator.
	if w.loading || w.session == nil {
		return w.renderLoading(width, height)
	}

	// If the cached render matches the current dimensions, reuse it.
	if w.rendered != "" && w.lastWidth == width && w.lastHeight == height {
		return w.applyOverlay(w.rendered, width, height)
	}

	// Render the image at the available interior size.
	imgW := width
	imgH := height
	if imgW < 1 {
		imgW = 1
	}
	if imgH < 1 {
		imgH = 1
	}

	rendered, err := w.renderer.RenderFile(w.session.ImagePath, imgW, imgH)
	if err != nil {
		w.err = err
		return w.renderError(width, height)
	}

	w.rendered = rendered
	w.lastWidth = width
	w.lastHeight = height
	w.loading = false

	return w.applyOverlay(rendered, width, height)
}

// MinSize returns the minimum width and height this widget requires.
func (w *WaifuWidget) MinSize() (int, int) {
	return 20, 10
}

// HandleKey processes key events when this widget has focus.
// 'r' triggers a refresh (random), 'i' toggles info overlay,
// 'n'/right navigates to next image, 'p'/left navigates to previous.
func (w *WaifuWidget) HandleKey(key tea.KeyMsg) tea.Cmd {
	switch key.Type {
	case tea.KeyRight:
		return w.navigateRelative(1)
	case tea.KeyLeft:
		return w.navigateRelative(-1)
	}

	switch key.String() {
	case "r":
		return w.refresh()
	case "i":
		w.showInfo = !w.showInfo
		return nil
	case "n":
		return w.navigateRelative(1)
	case "p":
		return w.navigateRelative(-1)
	}
	return nil
}

// refresh picks a new random image and resets the widget state.
func (w *WaifuWidget) refresh() tea.Cmd {
	if w.sessionMgr == nil {
		return nil
	}

	// Close the current session so GetOrCreate picks a new image.
	if w.session != nil {
		w.sessionMgr.Close(w.session.ID)
		w.session = nil
	}

	session, err := w.sessionMgr.GetOrCreate()
	if err != nil {
		w.err = err
		w.loading = false
		return nil
	}

	w.session = session
	w.overlayText = formatImageName(session.ImagePath)
	w.imageIndex = w.findImageIndex(session.ImagePath)
	w.rendered = ""
	w.lastWidth = 0
	w.lastHeight = 0
	w.loading = true
	w.err = nil

	return nil
}

// findImageIndex returns the index of path in the sorted imageList,
// or -1 if not found.
func (w *WaifuWidget) findImageIndex(path string) int {
	for i, p := range w.imageList {
		if p == path {
			return i
		}
	}
	return -1
}

// navigateTo switches to the image at the given index, wrapping around
// the image list boundaries.
func (w *WaifuWidget) navigateTo(idx int) tea.Cmd {
	if len(w.imageList) == 0 {
		return nil
	}

	// Wrap around.
	n := len(w.imageList)
	idx = ((idx % n) + n) % n

	imgPath := w.imageList[idx]
	w.imageIndex = idx

	// Create a new session for this image.
	w.session = &waifu.Session{
		ID:        fmt.Sprintf("ppulse-%d", os.Getpid()),
		ImagePath: imgPath,
	}
	w.overlayText = formatImageName(imgPath)
	w.rendered = ""
	w.lastWidth = 0
	w.lastHeight = 0
	w.loading = true
	w.err = nil

	return nil
}

// navigateRelative moves delta positions from the current image index.
// If the current index is unknown, starts from 0.
func (w *WaifuWidget) navigateRelative(delta int) tea.Cmd {
	if len(w.imageList) == 0 {
		return nil
	}
	base := w.imageIndex
	if base < 0 {
		base = 0
	}
	return w.navigateTo(base + delta)
}

// renderLoading creates a centered loading indicator.
func (w *WaifuWidget) renderLoading(width, height int) string {
	msg := "Loading..."
	return centerText(msg, width, height)
}

// renderError creates a centered error message.
func (w *WaifuWidget) renderError(width, height int) string {
	errMsg := w.err.Error()
	if components.VisibleLen(errMsg) > width-4 {
		errMsg = components.TruncateWithTail(errMsg, width-4, "...")
	}
	msg := components.Dim("[error] " + errMsg)
	return centerText(msg, width, height)
}

// applyOverlay composites the overlay text onto the bottom of the rendered
// image output when the info overlay is active. The overlay dims the last
// line(s) and places the character name/source on top.
func (w *WaifuWidget) applyOverlay(rendered string, width, height int) string {
	if !w.showInfo || w.overlayText == "" {
		return rendered
	}

	lines := strings.Split(rendered, "\n")

	// Build the overlay line: dim text, truncated to fit width.
	overlayContent := w.overlayText
	if components.VisibleLen(overlayContent) > width {
		overlayContent = components.TruncateWithTail(overlayContent, width, "...")
	}
	overlayLine := components.Dim(components.PadRight(overlayContent, width))

	// Replace the last line with the overlay.
	if len(lines) == 0 {
		lines = append(lines, overlayLine)
	} else {
		lines[len(lines)-1] = overlayLine
	}

	return strings.Join(lines, "\n")
}

// formatImageName extracts a human-readable name from an image file path.
// It strips the directory, removes the extension, and replaces underscores
// and hyphens with spaces for readability.
func formatImageName(path string) string {
	if path == "" {
		return ""
	}
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	return name
}

// centerText renders msg centered within a width x height area, padding
// with empty lines above and below.
func centerText(msg string, width, height int) string {
	// Center the message horizontally.
	centered := components.PadCenter(msg, width)

	var lines []string

	// Vertical centering: pad top.
	topPad := (height - 1) / 2
	if topPad < 0 {
		topPad = 0
	}
	emptyLine := strings.Repeat(" ", width)
	for i := 0; i < topPad; i++ {
		lines = append(lines, emptyLine)
	}

	lines = append(lines, centered)

	// Fill remaining height.
	for len(lines) < height {
		lines = append(lines, emptyLine)
	}

	// Truncate if we overshoot.
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

// SetError sets the widget into an error state. This is primarily useful
// for testing.
func (w *WaifuWidget) SetError(err error) {
	w.err = err
	w.loading = false
}

// SetRendered sets the cached rendered content and dimensions. This is
// primarily useful for testing cache reuse behavior.
func (w *WaifuWidget) SetRendered(rendered string, width, height int) {
	w.rendered = rendered
	w.lastWidth = width
	w.lastHeight = height
	w.loading = false
}

// SetSession sets the session directly, primarily for testing.
func (w *WaifuWidget) SetSession(session *waifu.Session) {
	w.session = session
	if session != nil {
		w.overlayText = formatImageName(session.ImagePath)
	}
}

// SetShowInfo sets the info overlay state, primarily for testing.
func (w *WaifuWidget) SetShowInfo(show bool) {
	w.showInfo = show
}

// SetLoading sets the loading state, primarily for testing.
func (w *WaifuWidget) SetLoading(loading bool) {
	w.loading = loading
}

// Rendered returns the cached rendered output for testing inspection.
func (w *WaifuWidget) Rendered() string {
	return w.rendered
}

// LastSize returns the last rendered dimensions for testing inspection.
func (w *WaifuWidget) LastSize() (int, int) {
	return w.lastWidth, w.lastHeight
}

// OverlayText returns the current overlay text for testing.
func (w *WaifuWidget) OverlayText() string {
	return w.overlayText
}

// ShowInfo returns whether the info overlay is active.
func (w *WaifuWidget) ShowInfo() bool {
	return w.showInfo
}

// ImageIndex returns the current position in the image list.
func (w *WaifuWidget) ImageIndex() int {
	return w.imageIndex
}

// ImageListLen returns the number of images in the navigation list.
func (w *WaifuWidget) ImageListLen() int {
	return len(w.imageList)
}

// SetImageList sets the image list and index for testing.
func (w *WaifuWidget) SetImageList(list []string, index int) {
	w.imageList = list
	w.imageIndex = index
}

// compile-time check that WaifuWidget implements app.Widget.
var _ app.Widget = (*WaifuWidget)(nil)

// compile-time check that ImageRenderer matches the waifu.ImageRenderer
// interface shape (both have RenderFile(string, int, int) (string, error)).
var _ ImageRenderer = (waifu.ImageRenderer)(nil)

// formatOverlayText is an alias kept for documentation; the actual
// formatting is done by formatImageName. This comment exists to note
// that the overlay text is the formatted image filename.
func init() {
	// Verify the format function works at init time with a trivial case.
	_ = formatImageName("")
}

// FormatImageName is the exported version of formatImageName for testing.
func FormatImageName(path string) string {
	return formatImageName(path)
}

// DataUpdateEventSource is the source string that the waifu collector
// uses when sending DataUpdateEvent messages. Widgets filter on this.
const DataUpdateEventSource = "waifu"

// CenterText is the exported version of centerText for testing.
func CenterText(msg string, width, height int) string {
	return centerText(msg, width, height)
}

// RenderLoading returns a loading indicator string at the given dimensions.
// Exported for testing.
func (w *WaifuWidget) RenderLoading(width, height int) string {
	return w.renderLoading(width, height)
}

// RenderError returns an error display string at the given dimensions.
// Exported for testing.
func (w *WaifuWidget) RenderError(width, height int) string {
	return w.renderError(width, height)
}
