package ArbiterTypes

import (
	"github.com/pion/webrtc/v3"
	"github.com/gorilla/websocket" 
	"sync"
)

type HandshakePayload struct {
	Type string `json:"type"`
	Sender string `json:"sender"`
	RemotePeer string `json:"remotePeerId"`
	Description webrtc.SessionDescription `json:"description,omitempty"`
	Candidate webrtc.ICECandidateInit `json:"candidate,omitempty"`
}

type ProducerTrackChannel struct {
	Id string `json:"id"`
	Track *webrtc.TrackRemote `json:"track"`
}

type SafeConnection struct {
	Conn *websocket.Conn
	Mu   sync.Mutex 
}