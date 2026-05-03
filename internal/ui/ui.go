package ui

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"netcat/internal/chat"

	"github.com/jroimartin/gocui"
)

type UIEventType int

const (
	UIEventChatMessage UIEventType = iota
	UIEventRoomsUpdate
	UIEventUsersUpdate
	UIEventQuit
	UIEventUsernameError
)

type UIEvent struct {
	Type  UIEventType
	Text  string
	Items []string
}

type UI struct {
	g           *gocui.Gui
	client      *UIClient
	events      chan UIEvent
	chatHistory []string
	rooms       []string
	users       []string
	currentRoom string
	errMessage  string
}

func NewUI() (*UI, error) {
	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		return nil, err
	}
	g.Cursor = true
	return &UI{
		g:           g,
		events:      make(chan UIEvent, 128),
		chatHistory: []string{},
		rooms:       []string{},
		users:       []string{},
	}, nil
}

func (ui *UI) Start(addr string) error {
	defer func() {
		ui.g.Close()
		if ui.errMessage != "" {
			fmt.Println(ui.errMessage)
		}
	}()

	ui.g.SetManagerFunc(ui.usernameLayout)
	if err := ui.setUsernameKeybindings(); err != nil {
		return err
	}

	client, err := NewUIClient(addr, ui)
	if err != nil {
		return err
	}
	ui.client = client

	go func() {
		_ = ui.Run()
	}()

	if err := ui.g.MainLoop(); err != nil && err != gocui.ErrQuit {
		return err
	}
	return nil
}

func (ui *UI) usernameLayout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if v, err := g.SetView("prompt", maxX/2-20, maxY/2-2, maxX/2+20, maxY/2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		fmt.Fprintln(v, "Enter your username:")
	}
	if v, err := g.SetView("username", maxX/2-20, maxY/2, maxX/2+20, maxY/2+2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Editable = true
		if _, err := g.SetCurrentView("username"); err != nil {
			return err
		}
	}
	return nil
}

func (ui *UI) chatLayout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	leftColumn := int(math.Max(math.Floor(0.2*float64(maxX)), 25))
	if v, err := g.SetView("rooms", 0, 0, leftColumn, maxY/2-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Rooms"
		ui.updateRoomsView()
	}
	if v, err := g.SetView("users", 0, maxY/2, leftColumn, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Users"
		ui.updateUsersView()
	}
	if v, err := g.SetView("chat", leftColumn+1, 0, maxX-1, maxY-4); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Chat"
		v.Autoscroll = true
		v.Wrap = true
		for _, msg := range ui.chatHistory {
			fmt.Fprintln(v, msg)
		}
	}
	if v, err := g.SetView("input", leftColumn+1, maxY-3, maxX-1, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Input"
		v.Editable = true
		if _, err := g.SetCurrentView("input"); err != nil {
			return err
		}
	}
	return nil
}

func (ui *UI) setUsernameKeybindings() error {
	if err := ui.g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}
	if err := ui.g.SetKeybinding("username", gocui.KeyEnter, gocui.ModNone, ui.submitUsername); err != nil {
		return err
	}
	return nil
}

func (ui *UI) setChatKeybindings() error {
	if err := ui.g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}
	if err := ui.g.SetKeybinding("input", gocui.KeyEnter, gocui.ModNone, ui.sendMessage); err != nil {
		return err
	}
	if err := ui.g.SetKeybinding("input", gocui.KeyArrowUp, gocui.ModNone, ui.scrollUp); err != nil {
		return err
	}
	if err := ui.g.SetKeybinding("input", gocui.KeyArrowDown, gocui.ModNone, ui.scrollDown); err != nil {
		return err
	}
	return nil
}

func (ui *UI) submitUsername(g *gocui.Gui, v *gocui.View) error {
	username := strings.TrimSpace(v.Buffer())
	if username == "" {
		return nil
	}
	if !utf8.ValidString(username) {
		return nil
	}
	if strings.HasPrefix(username, "/") {
		return nil
	}
	ui.client.username = username
	ui.client.SendUsername()
	v.Clear()
	v.SetCursor(0, 0)
	v.SetOrigin(0, 0)
	return nil
}

func (ui *UI) showChatLayout() {
	ui.g.Update(func(g *gocui.Gui) error {
		if _, err := g.View("input"); err == nil {
			return nil
		}
		g.DeleteView("prompt")
		g.DeleteView("username")
		ui.g.SetManagerFunc(ui.chatLayout)
		return ui.setChatKeybindings()
	})
}

func (ui *UI) sendMessage(g *gocui.Gui, v *gocui.View) error {
	msg := strings.TrimSpace(v.Buffer())
	if msg == "" {
		return nil
	}
	v.Clear()
	v.SetCursor(0, 0)
	v.SetOrigin(0, 0)
	if msg == "/history" {
		ui.showHistory()
	} else if msg == "/help" {
		ui.showHelp()
	} else {
		ui.client.SendMessage(msg)
	}
	return nil
}

func (ui *UI) scrollUp(g *gocui.Gui, v *gocui.View) error {
	cv, err := g.View("chat")
	if err != nil {
		return nil
	}

	cv.Autoscroll = false
	ox, oy := cv.Origin()
	if oy > 0 {
		cv.SetOrigin(ox, oy-1)
	}
	return nil
}

func (ui *UI) scrollDown(g *gocui.Gui, v *gocui.View) error {
	cv, err := g.View("chat")
	if err != nil {
		return nil
	}

	if cv.Autoscroll {
		return nil
	}

	ox, oy := cv.Origin()

	_, h := cv.Size()
	lines := cv.BufferLines()
	lineCount := len(lines)
	if lineCount > 0 && lines[lineCount-1] == "" {
		lineCount--
	}
	if oy+h >= lineCount {
		cv.Autoscroll = true
	} else {
		cv.SetOrigin(ox, oy+1)
	}
	return nil
}

func (ui *UI) Run() error {
	for ev := range ui.events {
		switch ev.Type {

		case UIEventChatMessage:
			ui.handleChatMessage(ev.Text)

		case UIEventRoomsUpdate:
			ui.handleRoomsUpdate(ev.Items)

		case UIEventUsersUpdate:
			ui.handleUsersUpdate(ev.Items)

		case UIEventUsernameError:
			ui.handleUsernameError(ev.Text)

		case UIEventQuit:
			ui.g.Update(func(g *gocui.Gui) error {
				return gocui.ErrQuit
			})
			// return gocui.ErrQuit
		}
	}
	return nil
}

func (ui *UI) PostChatMessage(msg string) {
	ui.events <- UIEvent{
		Type: UIEventChatMessage,
		Text: msg,
	}
}

func (ui *UI) PostRooms(rooms []string) {
	ui.events <- UIEvent{
		Type:  UIEventRoomsUpdate,
		Items: rooms,
	}
}

func (ui *UI) PostUsers(users []string) {
	ui.events <- UIEvent{
		Type:  UIEventUsersUpdate,
		Items: users,
	}
}

func (ui *UI) PostUsernameError(message string) {
	ui.events <- UIEvent{
		Type: UIEventUsernameError,
		Text: message,
	}
}

func (ui *UI) Quit() {
	ui.events <- UIEvent{Type: UIEventQuit}
}

func (ui *UI) handleChatMessage(msg string) {
	ui.chatHistory = append(ui.chatHistory, msg)
	if len(ui.chatHistory) > 64 {
		ui.chatHistory = ui.chatHistory[1:]
	}
	ui.g.Update(func(g *gocui.Gui) error {
		v, err := g.View("chat")
		if err != nil {
			return nil
		}
		fmt.Fprintln(v, msg)
		return nil
	})
}

func (ui *UI) SetRooms(rooms []string) {
	ui.rooms = rooms
}

func (ui *UI) handleRoomsUpdate(rooms []string) {
	ui.rooms = append([]string(nil), rooms...)
	ui.g.Update(func(g *gocui.Gui) error {
		ui.updateRoomsView()
		g.SetCurrentView("input")
		return nil
	})
}

func (ui *UI) handleUsersUpdate(users []string) {
	ui.users = append([]string(nil), users...)
	ui.g.Update(func(g *gocui.Gui) error {
		ui.updateUsersView()
		g.SetCurrentView("input")
		return nil
	})
}

func (ui *UI) updateRoomsView() {
	v, err := ui.g.View("rooms")
	if err != nil {
		return
	}
	v.Clear()
	for _, room := range ui.rooms {
		fmt.Fprintln(v, room)
	}
}

func (ui *UI) updateUsersView() {
	v, err := ui.g.View("users")
	if err != nil {
		return
	}
	v.Clear()
	for _, user := range ui.users {
		fmt.Fprintln(v, user)
	}
}

func (ui *UI) showHistory() {
	ui.g.Update(func(g *gocui.Gui) error {
		v, err := g.View("chat")
		if err != nil {
			return nil
		}
		fmt.Fprintln(v, "=== Chat History (last 64 messages) ===")
		for _, msg := range ui.chatHistory {
			fmt.Fprintln(v, msg)
		}
		fmt.Fprintln(v, "=== End History ===")
		return nil
	})
}

func (ui *UI) showHelp() {
	ui.g.Update(func(g *gocui.Gui) error {
		v, err := g.View("chat")
		if err != nil {
			return nil
		}
		for _, line := range chat.GetHelpCommands() {
			fmt.Fprintln(v, line)
		}
		return nil
	})
}

func (ui *UI) handleUsernameError(message string) {
	ui.errMessage = message
	ui.g.Update(func(g *gocui.Gui) error {
		return gocui.ErrQuit
	})
}

func (ui *UI) requestQuit() {
	ui.Quit()
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
