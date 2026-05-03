package ui

import (
	"testing"
)

func TestUIPostEvents(t *testing.T) {
	u, err := NewUI()
	if err != nil {
		t.Fatalf("NewUI failed: %v", err)
	}
	defer u.g.Close()

	u.PostChatMessage("hello")
	ev := <-u.events
	if ev.Type != UIEventChatMessage || ev.Text != "hello" {
		t.Errorf("Unexpected event: %+v", ev)
	}

	u.PostRooms([]string{"RoomA"})
	ev = <-u.events
	if ev.Type != UIEventRoomsUpdate || len(ev.Items) != 1 || ev.Items[0] != "RoomA" {
		t.Errorf("Unexpected event: %+v", ev)
	}

	u.PostUsers([]string{"UserA"})
	ev = <-u.events
	if ev.Type != UIEventUsersUpdate || len(ev.Items) != 1 || ev.Items[0] != "UserA" {
		t.Errorf("Unexpected event: %+v", ev)
	}

	u.PostUsernameError("bad user")
	ev = <-u.events
	if ev.Type != UIEventUsernameError || ev.Text != "bad user" {
		t.Errorf("Unexpected event: %+v", ev)
	}

	u.Quit()
	ev = <-u.events
	if ev.Type != UIEventQuit {
		t.Errorf("Unexpected event: %+v", ev)
	}
}

func TestUISetRooms(t *testing.T) {
	u, err := NewUI()
	if err != nil {
		t.Fatalf("NewUI failed: %v", err)
	}
	defer u.g.Close()

	u.SetRooms([]string{"r1", "r2"})
	if len(u.rooms) != 2 || u.rooms[0] != "r1" || u.rooms[1] != "r2" {
		t.Errorf("SetRooms failed, rooms: %v", u.rooms)
	}
}
