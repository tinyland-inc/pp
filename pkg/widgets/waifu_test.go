package widgets

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gitlab.com/tinyland/lab/prompt-pulse/pkg/app"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/waifu"
)

// mockRenderer is a test double for ImageRenderer that returns a fixed
// string or an error.
type mockRenderer struct {
	output string
	err    error
	calls  int
}

func (m *mockRenderer) RenderFile(path string, width, height int) (string, error) {
	m.calls++
	if m.err != nil {
		return "", m.err
	}
	if m.output != "" {
		return m.output, nil
	}
	// Generate a placeholder grid of the requested size.
	var lines []string
	for y := 0; y < height; y++ {
		lines = append(lines, strings.Repeat("#", width))
	}
	return strings.Join(lines, "\n"), nil
}

// newTestSession creates a Session for testing purposes.
func newTestSession(imgPath string) *waifu.Session {
	return &waifu.Session{
		ID:          "ppulse-test",
		ImagePath:   imgPath,
		ContentHash: "abcdef1234567890",
		CreatedAt:   time.Now(),
	}
}

// newTestWidget creates a WaifuWidget with a mock renderer and a
// pre-populated session, bypassing the SessionManager.
func newTestWidget(renderer *mockRenderer, session *waifu.Session) *WaifuWidget {
	w := &WaifuWidget{
		id:       "waifu",
		renderer: renderer,
		loading:  false,
	}
	if session != nil {
		w.session = session
		w.overlayText = formatImageName(session.ImagePath)
	}
	return w
}

func TestWaifuID(t *testing.T) {
	w := newTestWidget(&mockRenderer{}, nil)
	if got := w.ID(); got != "waifu" {
		t.Errorf("ID() = %q, want %q", got, "waifu")
	}
}

func TestTitleDefault(t *testing.T) {
	w := newTestWidget(&mockRenderer{}, nil)
	if got := w.Title(); got != "Waifu" {
		t.Errorf("Title() = %q, want %q", got, "Waifu")
	}
}

func TestTitleWithSession(t *testing.T) {
	session := newTestSession("/images/sakura_bloom.png")
	w := newTestWidget(&mockRenderer{}, session)
	// No imageList, so no index indicator.
	want := "sakura bloom"
	if got := w.Title(); got != want {
		t.Errorf("Title() = %q, want %q", got, want)
	}
}

func TestTitleWithIndexIndicator(t *testing.T) {
	session := newTestSession("/images/b_image.png")
	w := newTestWidget(&mockRenderer{}, session)
	w.SetImageList([]string{"/images/a_image.png", "/images/b_image.png", "/images/c_image.png"}, 1)
	want := "b image [2/3]"
	if got := w.Title(); got != want {
		t.Errorf("Title() = %q, want %q", got, want)
	}
}

func TestWaifuMinSize(t *testing.T) {
	w := newTestWidget(&mockRenderer{}, nil)
	gotW, gotH := w.MinSize()
	if gotW != 20 || gotH != 10 {
		t.Errorf("MinSize() = (%d, %d), want (20, 10)", gotW, gotH)
	}
}

func TestViewLoadingState(t *testing.T) {
	w := newTestWidget(&mockRenderer{}, nil)
	w.SetLoading(true)
	view := w.View(40, 20)
	if !strings.Contains(view, "Loading...") {
		t.Errorf("View() in loading state should contain 'Loading...', got:\n%s", view)
	}
}

func TestViewNoSessionShowsLoading(t *testing.T) {
	w := newTestWidget(&mockRenderer{}, nil)
	w.session = nil
	w.loading = false
	view := w.View(40, 20)
	// With no session and not loading, it should still show loading
	// because session == nil triggers the loading branch.
	if !strings.Contains(view, "Loading...") {
		t.Errorf("View() with nil session should contain 'Loading...', got:\n%s", view)
	}
}

func TestViewErrorState(t *testing.T) {
	w := newTestWidget(&mockRenderer{}, nil)
	w.SetError(errors.New("image directory not found"))
	view := w.View(60, 20)
	if !strings.Contains(view, "error") {
		t.Errorf("View() in error state should contain 'error', got:\n%s", view)
	}
	if !strings.Contains(view, "image directory not found") {
		t.Errorf("View() in error state should contain error message, got:\n%s", view)
	}
}

func TestViewRendersImage(t *testing.T) {
	session := newTestSession("/images/test.png")
	renderer := &mockRenderer{output: "RENDERED_IMAGE_OUTPUT"}
	w := newTestWidget(renderer, session)

	view := w.View(40, 20)
	if !strings.Contains(view, "RENDERED_IMAGE_OUTPUT") {
		t.Errorf("View() should contain rendered output, got:\n%s", view)
	}
	if renderer.calls != 1 {
		t.Errorf("Renderer called %d times, want 1", renderer.calls)
	}
}

func TestViewVariousSizes(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"minimum", 20, 10},
		{"medium", 40, 20},
		{"large", 80, 40},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := newTestSession("/images/test.png")
			renderer := &mockRenderer{}
			w := newTestWidget(renderer, session)

			view := w.View(tt.width, tt.height)
			if view == "" {
				t.Errorf("View(%d, %d) returned empty string", tt.width, tt.height)
			}
			if renderer.calls != 1 {
				t.Errorf("Renderer called %d times, want 1", renderer.calls)
			}
		})
	}
}

func TestViewZeroDimensions(t *testing.T) {
	w := newTestWidget(&mockRenderer{}, nil)
	if got := w.View(0, 0); got != "" {
		t.Errorf("View(0, 0) = %q, want empty string", got)
	}
	if got := w.View(-1, 10); got != "" {
		t.Errorf("View(-1, 10) = %q, want empty string", got)
	}
}

func TestViewCachedRenderReuse(t *testing.T) {
	session := newTestSession("/images/test.png")
	renderer := &mockRenderer{output: "CACHED_OUTPUT"}
	w := newTestWidget(renderer, session)

	// First render.
	_ = w.View(40, 20)
	if renderer.calls != 1 {
		t.Fatalf("Expected 1 render call after first View, got %d", renderer.calls)
	}

	// Second render at same dimensions should reuse cache.
	view2 := w.View(40, 20)
	if renderer.calls != 1 {
		t.Errorf("Expected no additional render call for same dimensions, got %d total calls", renderer.calls)
	}
	if !strings.Contains(view2, "CACHED_OUTPUT") {
		t.Errorf("Cached view should contain 'CACHED_OUTPUT', got:\n%s", view2)
	}
}

func TestViewCacheInvalidatedOnResize(t *testing.T) {
	session := newTestSession("/images/test.png")
	renderer := &mockRenderer{output: "OUTPUT"}
	w := newTestWidget(renderer, session)

	_ = w.View(40, 20)
	if renderer.calls != 1 {
		t.Fatalf("Expected 1 call, got %d", renderer.calls)
	}

	// Different dimensions should trigger re-render.
	_ = w.View(80, 40)
	if renderer.calls != 2 {
		t.Errorf("Expected 2 calls after size change, got %d", renderer.calls)
	}
}

func TestHandleKeyRefresh(t *testing.T) {
	session := newTestSession("/images/test.png")
	renderer := &mockRenderer{output: "OUTPUT"}
	w := newTestWidget(renderer, session)
	w.SetRendered("OUTPUT", 40, 20)

	// 'r' should trigger refresh logic (returns nil cmd since we have no
	// sessionMgr, but the key should be consumed).
	cmd := w.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	// Without a session manager, refresh returns nil.
	if cmd != nil {
		t.Errorf("HandleKey('r') without sessionMgr should return nil, got %v", cmd)
	}
}

func TestHandleKeyInfo(t *testing.T) {
	session := newTestSession("/images/test.png")
	w := newTestWidget(&mockRenderer{}, session)

	if w.ShowInfo() {
		t.Error("showInfo should default to false")
	}

	_ = w.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if !w.ShowInfo() {
		t.Error("showInfo should be true after pressing 'i'")
	}

	_ = w.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if w.ShowInfo() {
		t.Error("showInfo should toggle back to false")
	}
}

func TestHandleKeyUnknown(t *testing.T) {
	w := newTestWidget(&mockRenderer{}, nil)
	cmd := w.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	if cmd != nil {
		t.Errorf("HandleKey('z') should return nil for unhandled key")
	}
}

func TestUpdateDataUpdateEvent(t *testing.T) {
	w := newTestWidget(&mockRenderer{}, nil)
	w.SetLoading(true)

	// Simulate receiving rendered data from the waifu collector.
	evt := app.DataUpdateEvent{
		Source:    "waifu",
		Data:      "RENDERED_FROM_COLLECTOR",
		Timestamp: time.Now(),
	}

	cmd := w.Update(evt)
	if cmd != nil {
		t.Errorf("Update(DataUpdateEvent) should return nil, got %v", cmd)
	}
	if w.Rendered() != "RENDERED_FROM_COLLECTOR" {
		t.Errorf("rendered = %q, want %q", w.Rendered(), "RENDERED_FROM_COLLECTOR")
	}
	if w.loading {
		t.Error("loading should be false after receiving data")
	}
}

func TestUpdateDataUpdateEventWithSession(t *testing.T) {
	w := newTestWidget(&mockRenderer{}, nil)

	session := newTestSession("/images/new_character.png")
	evt := app.DataUpdateEvent{
		Source:    "waifu",
		Data:      session,
		Timestamp: time.Now(),
	}

	w.Update(evt)
	if w.session != session {
		t.Error("session should be updated")
	}
	if w.OverlayText() != "new character" {
		t.Errorf("overlayText = %q, want %q", w.OverlayText(), "new character")
	}
	if !w.loading {
		t.Error("loading should be true after receiving a new session (needs render)")
	}
}

func TestUpdateDataUpdateEventWithError(t *testing.T) {
	w := newTestWidget(&mockRenderer{}, nil)

	evt := app.DataUpdateEvent{
		Source:    "waifu",
		Data:      nil,
		Err:       errors.New("fetch failed"),
		Timestamp: time.Now(),
	}

	w.Update(evt)
	if w.err == nil || w.err.Error() != "fetch failed" {
		t.Errorf("err = %v, want 'fetch failed'", w.err)
	}
}

func TestUpdateIgnoresOtherSources(t *testing.T) {
	w := newTestWidget(&mockRenderer{}, nil)
	w.SetLoading(true)

	evt := app.DataUpdateEvent{
		Source:    "tailscale",
		Data:      "some data",
		Timestamp: time.Now(),
	}

	w.Update(evt)
	// Should remain in loading state since the event was for a different source.
	if !w.loading {
		t.Error("loading should remain true for events from other sources")
	}
}

func TestUpdateWindowSizeMsg(t *testing.T) {
	session := newTestSession("/images/test.png")
	renderer := &mockRenderer{output: "OUTPUT"}
	w := newTestWidget(renderer, session)
	w.SetRendered("OUTPUT", 40, 20)

	// Window resize should invalidate the cache.
	w.Update(tea.WindowSizeMsg{Width: 80, Height: 40})

	lastW, lastH := w.LastSize()
	if lastW != 0 || lastH != 0 {
		t.Errorf("LastSize() = (%d, %d), want (0, 0) after resize", lastW, lastH)
	}
}

func TestOverlayTextFormatting(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/images/sakura_bloom.png", "sakura bloom"},
		{"/path/to/dark-knight.jpg", "dark knight"},
		{"/anime/character_name-series.gif", "character name series"},
		{"simple.png", "simple"},
		{"", ""},
		{"/dir/UPPERCASE.PNG", "UPPERCASE"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := FormatImageName(tt.path)
			if got != tt.want {
				t.Errorf("FormatImageName(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestOverlayAppliedInView(t *testing.T) {
	session := newTestSession("/images/test_character.png")
	// Create a renderer that returns multi-line output.
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, strings.Repeat("X", 40))
	}
	renderer := &mockRenderer{output: strings.Join(lines, "\n")}
	w := newTestWidget(renderer, session)
	w.SetShowInfo(true)

	view := w.View(40, 20)
	// The last line should contain the overlay text.
	viewLines := strings.Split(view, "\n")
	lastLine := viewLines[len(viewLines)-1]
	if !strings.Contains(lastLine, "test character") {
		t.Errorf("Last line with info overlay should contain 'test character', got:\n%s", lastLine)
	}
}

func TestOverlayNotAppliedWhenInfoOff(t *testing.T) {
	session := newTestSession("/images/test_character.png")
	renderer := &mockRenderer{output: "SIMPLE_OUTPUT"}
	w := newTestWidget(renderer, session)
	w.SetShowInfo(false)

	view := w.View(40, 20)
	if strings.Contains(view, "test character") {
		t.Errorf("View with showInfo=false should not contain overlay text")
	}
}

func TestCenterText(t *testing.T) {
	result := CenterText("hello", 20, 5)
	lines := strings.Split(result, "\n")
	if len(lines) != 5 {
		t.Errorf("CenterText should produce %d lines, got %d", 5, len(lines))
	}
	// The message should appear in the middle line (index 2 for height 5).
	middleLine := lines[2]
	if !strings.Contains(middleLine, "hello") {
		t.Errorf("Middle line should contain 'hello', got %q", middleLine)
	}
}

func TestViewRendererError(t *testing.T) {
	session := newTestSession("/images/test.png")
	renderer := &mockRenderer{err: errors.New("render failed")}
	w := newTestWidget(renderer, session)

	view := w.View(40, 20)
	if !strings.Contains(view, "render failed") {
		t.Errorf("View should show renderer error, got:\n%s", view)
	}
}

func TestWidgetImplementsInterface(t *testing.T) {
	// This is a compile-time check via the var _ line in waifu.go,
	// but we also verify at runtime.
	var w app.Widget = newTestWidget(&mockRenderer{}, nil)
	if w.ID() != "waifu" {
		t.Error("Widget interface implementation broken")
	}
}

func TestSetRenderedAndCacheReuse(t *testing.T) {
	session := newTestSession("/images/test.png")
	renderer := &mockRenderer{}
	w := newTestWidget(renderer, session)

	// Pre-populate the cache via SetRendered.
	w.SetRendered("PRE_CACHED", 40, 20)

	view := w.View(40, 20)
	if renderer.calls != 0 {
		t.Errorf("Expected 0 render calls when cache is pre-populated, got %d", renderer.calls)
	}
	// The view should contain the pre-cached content (possibly with overlay).
	if !strings.Contains(view, "PRE_CACHED") {
		t.Errorf("View should return pre-cached content, got:\n%s", view)
	}
}

func TestDataUpdateEventSource(t *testing.T) {
	if DataUpdateEventSource != "waifu" {
		t.Errorf("DataUpdateEventSource = %q, want %q", DataUpdateEventSource, "waifu")
	}
}

func TestRenderLoadingDimensions(t *testing.T) {
	w := newTestWidget(&mockRenderer{}, nil)
	result := w.RenderLoading(30, 10)
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Errorf("RenderLoading should produce %d lines, got %d", 10, len(lines))
	}
}

func TestRenderErrorDimensions(t *testing.T) {
	w := newTestWidget(&mockRenderer{}, nil)
	w.SetError(errors.New("test error"))
	result := w.RenderError(30, 10)
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Errorf("RenderError should produce %d lines, got %d", 10, len(lines))
	}
}

func TestHandleKeyNext(t *testing.T) {
	session := newTestSession("/images/b.png")
	w := newTestWidget(&mockRenderer{output: "IMG"}, session)
	w.SetImageList([]string{"/images/a.png", "/images/b.png", "/images/c.png"}, 1)

	_ = w.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if w.ImageIndex() != 2 {
		t.Errorf("after 'n': ImageIndex() = %d, want 2", w.ImageIndex())
	}
	if w.session == nil || w.session.ImagePath != "/images/c.png" {
		t.Errorf("after 'n': ImagePath = %q, want /images/c.png", w.session.ImagePath)
	}
}

func TestHandleKeyPrev(t *testing.T) {
	session := newTestSession("/images/b.png")
	w := newTestWidget(&mockRenderer{output: "IMG"}, session)
	w.SetImageList([]string{"/images/a.png", "/images/b.png", "/images/c.png"}, 1)

	_ = w.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if w.ImageIndex() != 0 {
		t.Errorf("after 'p': ImageIndex() = %d, want 0", w.ImageIndex())
	}
	if w.session == nil || w.session.ImagePath != "/images/a.png" {
		t.Errorf("after 'p': ImagePath = %q, want /images/a.png", w.session.ImagePath)
	}
}

func TestNavigationWrapsForward(t *testing.T) {
	session := newTestSession("/images/c.png")
	w := newTestWidget(&mockRenderer{output: "IMG"}, session)
	w.SetImageList([]string{"/images/a.png", "/images/b.png", "/images/c.png"}, 2)

	_ = w.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if w.ImageIndex() != 0 {
		t.Errorf("wrap forward: ImageIndex() = %d, want 0", w.ImageIndex())
	}
}

func TestNavigationWrapsBackward(t *testing.T) {
	session := newTestSession("/images/a.png")
	w := newTestWidget(&mockRenderer{output: "IMG"}, session)
	w.SetImageList([]string{"/images/a.png", "/images/b.png", "/images/c.png"}, 0)

	_ = w.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if w.ImageIndex() != 2 {
		t.Errorf("wrap backward: ImageIndex() = %d, want 2", w.ImageIndex())
	}
}

func TestNavigationArrowKeys(t *testing.T) {
	session := newTestSession("/images/b.png")
	w := newTestWidget(&mockRenderer{output: "IMG"}, session)
	w.SetImageList([]string{"/images/a.png", "/images/b.png", "/images/c.png"}, 1)

	_ = w.HandleKey(tea.KeyMsg{Type: tea.KeyRight})
	if w.ImageIndex() != 2 {
		t.Errorf("after Right: ImageIndex() = %d, want 2", w.ImageIndex())
	}

	_ = w.HandleKey(tea.KeyMsg{Type: tea.KeyLeft})
	if w.ImageIndex() != 1 {
		t.Errorf("after Left: ImageIndex() = %d, want 1", w.ImageIndex())
	}
}

func TestNavigationNoImages(t *testing.T) {
	w := newTestWidget(&mockRenderer{}, nil)
	cmd := w.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd != nil {
		t.Error("navigation with no images should return nil")
	}
}
