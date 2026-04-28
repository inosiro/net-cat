package chat

const MaxRooms = 256
const MaxRoomHistory = 64

type Room struct {
	Name string

	Messages chan ChatMessage

	events chan roomEvent
	done   chan struct{}

	clients map[string]*Client
	history []ChatMessage
}

type roomEventType int

const (
	roomJoin roomEventType = iota
	roomLeave
	roomDisconnect
	roomShutdown
	roomNickChange
	roomGetClients
	roomClientCount
)

type roomEvent struct {
	typ      roomEventType
	client   *Client
	oldName  string
	newName  string
	respChan chan interface{}
}

// NewRoom creates a new room with the given name.
func NewRoom(name string) *Room {
	return &Room{
		Name:     name,
		Messages: make(chan ChatMessage, 128),
		events:   make(chan roomEvent, 64),
		done:     make(chan struct{}),
		clients:  make(map[string]*Client),
		history:  make([]ChatMessage, 0, MaxRoomHistory),
	}
}

func (r *Room) Join(c *Client) {
	r.events <- roomEvent{typ: roomJoin, client: c}
}

func (r *Room) Leave(c *Client) {
	r.events <- roomEvent{typ: roomLeave, client: c}
}

func (r *Room) Disconnect(c *Client) {
	r.events <- roomEvent{typ: roomDisconnect, client: c}
}

func (r *Room) Shutdown() {
	r.events <- roomEvent{typ: roomShutdown}
}

func (r *Room) ChangeNick(c *Client, oldName, newName string) {
	r.events <- roomEvent{typ: roomNickChange, client: c, oldName: oldName, newName: newName}
}

// ClientCount returns the number of clients in the room.
func (r *Room) ClientCount() int {
	resp := make(chan interface{})
	r.events <- roomEvent{typ: roomClientCount, respChan: resp}
	count := <-resp
	return count.(int)
}

// GetClients returns a list of client usernames in the room.
func (r *Room) GetClients() []string {
	resp := make(chan interface{})
	r.events <- roomEvent{typ: roomGetClients, respChan: resp}
	clients := <-resp
	return clients.([]string)
}
