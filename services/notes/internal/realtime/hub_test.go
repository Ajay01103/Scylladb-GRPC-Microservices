package realtime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Ajay01103/go-notion/notes/internal/service"
)

func newTestRoom() *room {
	logger := zap.NewNop()
	return &room{
		noteID:    uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		svc:       nil,
		logger:    logger,
		clients:   make(map[*client]struct{}),
		join:      make(chan *client, 32),
		leave:     make(chan *client, 32),
		broadcast: make(chan roomMessage, 256),
		onEmpty:   func() {},
		buffer:    make([]service.BufferedUpdate, 0, 128),
	}
}

func newTestClient(uid uuid.UUID) *client {
	return &client{
		conn:   nil,
		send:   make(chan []byte, 64),
		userID: uid,
	}
}

func readChan(ch <-chan []byte, timeout time.Duration) []byte {
	select {
	case msg := <-ch:
		return msg
	case <-time.After(timeout):
		return nil
	}
}

func TestIdentifyBroadcastsPresenceToOthers(t *testing.T) {
	r := newTestRoom()
	go r.run()
	defer func() { r.leave <- nil }()

	alice := newTestClient(uuid.New())
	bob := newTestClient(uuid.New())

	r.join <- alice
	r.join <- bob

	// Drain init and presence_list messages.
	drain(alice.send, 100*time.Millisecond)
	drain(bob.send, 100*time.Millisecond)

	identifyPayload, _ := json.Marshal(identifyMessage{
		Type:   "identify",
		Name:   "Alice",
		Color:  "#ff0000",
		Avatar: "https://example.com/alice.png",
	})

	r.broadcast <- roomMessage{data: identifyPayload, sender: alice, userID: alice.userID}

	// Alice should NOT receive her own identify back as a raw frame,
	// nor a presence message about herself.
	if msg := readChan(alice.send, 100*time.Millisecond); msg != nil {
		t.Fatalf("alice should not receive anything after identify, got: %s", string(msg))
	}

	// Bob should receive a presence message about Alice joining.
	msg := readChan(bob.send, 100*time.Millisecond)
	if msg == nil {
		t.Fatal("bob should have received presence message after alice identified")
	}

	var pr presenceMessage
	if err := json.Unmarshal(msg, &pr); err != nil {
		t.Fatalf("failed to unmarshal presence: %v", err)
	}
	if pr.Type != "presence" {
		t.Fatalf("expected type presence, got %s", pr.Type)
	}
	if pr.UserID != alice.userID.String() {
		t.Fatalf("expected userID %s, got %s", alice.userID.String(), pr.UserID)
	}
	if pr.Name != "Alice" {
		t.Fatalf("expected name Alice, got %s", pr.Name)
	}
	if pr.Color != "#ff0000" {
		t.Fatalf("expected color #ff0000, got %s", pr.Color)
	}
	if pr.Avatar != "https://example.com/alice.png" {
		t.Fatalf("expected avatar, got %s", pr.Avatar)
	}
	if !pr.Joined {
		t.Fatal("expected Joined=true")
	}

	// Verify alice's client state was updated.
	if alice.name != "Alice" {
		t.Fatalf("alice name not stored: got %s", alice.name)
	}
	if alice.color != "#ff0000" {
		t.Fatalf("alice color not stored: got %s", alice.color)
	}

	// Clean up remaining goroutine.
	if bob != nil {
		r.leave <- bob
		drain(bob.send, 100*time.Millisecond)
	}
	r.leave <- alice
	drain(alice.send, 100*time.Millisecond)
}

func TestPresenceListOnJoin(t *testing.T) {
	r := newTestRoom()
	go r.run()
	defer func() { r.leave <- nil }()

	alice := newTestClient(uuid.New())
	alice.name = "Alice"
	alice.color = "#ff0000"
	alice.avatar = "https://a.com"

	bob := newTestClient(uuid.New())
	bob.name = "Bob"
	bob.color = "#00ff00"

	charlie := newTestClient(uuid.New())

	r.join <- alice
	r.join <- bob
	r.join <- charlie

	// Drain init messages.
	drain(alice.send, 100*time.Millisecond)
	drain(bob.send, 100*time.Millisecond)
	drain(charlie.send, 100*time.Millisecond)

	// Charlie joined after Alice and Bob identified.
	// Charlie should receive a presence_list with Alice and Bob.
	var foundList bool
	for {
		msg := readChan(charlie.send, 100*time.Millisecond)
		if msg == nil {
			break
		}
		var envelope struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal(msg, &envelope)
		if envelope.Type == "presence_list" {
			foundList = true
			var pl presenceListMessage
			if err := json.Unmarshal(msg, &pl); err != nil {
				t.Fatalf("failed to unmarshal presence_list: %v", err)
			}
			if len(pl.Users) != 2 {
				t.Fatalf("expected 2 users in presence_list, got %d", len(pl.Users))
			}
			uids := map[string]bool{}
			for _, u := range pl.Users {
				uids[u.UserID] = true
				if u.UserID == alice.userID.String() {
					if u.Name != "Alice" || u.Color != "#ff0000" || u.Avatar != "https://a.com" {
						t.Fatalf("alice info mismatch: %+v", u)
					}
				}
				if u.UserID == bob.userID.String() {
					if u.Name != "Bob" || u.Color != "#00ff00" {
						t.Fatalf("bob info mismatch: %+v", u)
					}
				}
			}
			if !uids[alice.userID.String()] || !uids[bob.userID.String()] {
				t.Fatal("presence_list missing expected users")
			}
		}
	}
	if !foundList {
		t.Fatal("charlie did not receive presence_list")
	}

	r.leave <- charlie
	r.leave <- bob
	r.leave <- alice
}

func TestLeaveBroadcastsPresenceIfIdentified(t *testing.T) {
	r := newTestRoom()
	go r.run()
	defer func() { r.leave <- nil }()

	alice := newTestClient(uuid.New())
	alice.name = "Alice"
	alice.color = "#ff0000"

	bob := newTestClient(uuid.New())

	r.join <- alice
	r.join <- bob

	drain(alice.send, 100*time.Millisecond)
	drain(bob.send, 100*time.Millisecond)

	// Bob leaves without identifying — no presence broadcast.
	r.leave <- bob

	// Alice should not receive anything.
	if msg := readChan(alice.send, 100*time.Millisecond); msg != nil {
		t.Fatalf("alice should not receive leave presence for unidentified bob, got: %s", string(msg))
	}

	// Now alice leaves — since she identified, but bob is gone, no one receives it.
	r.leave <- alice
}

func TestLeaveBroadcastsPresenceToRemaining(t *testing.T) {
	r := newTestRoom()
	go r.run()
	defer func() { r.leave <- nil }()

	alice := newTestClient(uuid.New())
	alice.name = "Alice"
	alice.color = "#ff0000"

	bob := newTestClient(uuid.New())
	bob.name = "Bob"
	bob.color = "#00ff00"

	r.join <- alice
	r.join <- bob

	drain(alice.send, 100*time.Millisecond)
	drain(bob.send, 100*time.Millisecond)

	// Alice leaves — Bob should receive leave presence.
	r.leave <- alice

	msg := readChan(bob.send, 100*time.Millisecond)
	if msg == nil {
		t.Fatal("bob should have received leave presence for alice")
	}

	var pr presenceMessage
	if err := json.Unmarshal(msg, &pr); err != nil {
		t.Fatalf("failed to unmarshal presence: %v", err)
	}
	if pr.Type != "presence" {
		t.Fatalf("expected type presence, got %s", pr.Type)
	}
	if pr.UserID != alice.userID.String() {
		t.Fatalf("expected userID %s, got %s", alice.userID.String(), pr.UserID)
	}
	if pr.Joined {
		t.Fatal("expected Joined=false for leave")
	}
	if pr.Name != "Alice" {
		t.Fatalf("expected name Alice, got %s", pr.Name)
	}

	// Clean up
	r.leave <- bob
	drain(bob.send, 100*time.Millisecond)
}

func TestCursorForwardedToOthersWithSenderUserID(t *testing.T) {
	r := newTestRoom()
	go r.run()
	defer func() { r.leave <- nil }()

	alice := newTestClient(uuid.New())
	bob := newTestClient(uuid.New())

	r.join <- alice
	r.join <- bob

	drain(alice.send, 100*time.Millisecond)
	drain(bob.send, 100*time.Millisecond)

	cursorPayload, _ := json.Marshal(cursorMessage{
		Type: "cursor",
		UserID: "spoofed", // should be overwritten by server
		X:    12.5,
		Y:    34.7,
	})

	r.broadcast <- roomMessage{data: cursorPayload, sender: alice, userID: alice.userID}

	// Alice should NOT receive her own cursor back.
	if msg := readChan(alice.send, 100*time.Millisecond); msg != nil {
		t.Fatalf("alice should not receive cursor echo, got: %s", string(msg))
	}

	// Bob should receive cursor with alice's userID.
	msg := readChan(bob.send, 100*time.Millisecond)
	if msg == nil {
		t.Fatal("bob should have received cursor message")
	}

	var cur cursorMessage
	if err := json.Unmarshal(msg, &cur); err != nil {
		t.Fatalf("failed to unmarshal cursor: %v", err)
	}
	if cur.Type != "cursor" {
		t.Fatalf("expected type cursor, got %s", cur.Type)
	}
	if cur.UserID != alice.userID.String() {
		t.Fatalf("expected userID %s, got %s", alice.userID.String(), cur.UserID)
	}
	if cur.X != 12.5 || cur.Y != 34.7 {
		t.Fatalf("cursor coords mismatch: x=%v y=%v", cur.X, cur.Y)
	}

	r.leave <- bob
	r.leave <- alice
}

func TestPatchStillBroadcastsRawFrame(t *testing.T) {
	r := newTestRoom()
	go r.run()
	defer func() { r.leave <- nil }()

	alice := newTestClient(uuid.New())
	bob := newTestClient(uuid.New())

	r.join <- alice
	r.join <- bob

	drain(alice.send, 100*time.Millisecond)
	drain(bob.send, 100*time.Millisecond)

	patchPayload, _ := json.Marshal(patchMessage{
		Type:     "patch",
		Upserted: map[string]json.RawMessage{"block1": json.RawMessage(`{"text":"hi"}`)},
	})

	r.broadcast <- roomMessage{data: patchPayload, sender: alice, userID: alice.userID}

	// Bob should receive the raw patch frame.
	msg := readChan(bob.send, 100*time.Millisecond)
	if msg == nil {
		t.Fatal("bob should have received patch")
	}
	if string(msg) != string(patchPayload) {
		t.Fatalf("patch payload mismatch: got %s", string(msg))
	}

	r.leave <- bob
	r.leave <- alice
}

func drain(ch <-chan []byte, timeout time.Duration) {
	for {
		select {
		case <-ch:
		case <-time.After(timeout):
			return
		}
	}
}
