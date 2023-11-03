package ArbiterTypes

import (
	"github.com/pion/webrtc/v3"
)

type HandshakePayload struct {
	Type string `json:"type"`
	Sender string `json:"sender"`
	Description webrtc.SessionDescription `json:"description,omitempty"`
	Candidate webrtc.ICECandidateInit `json:"candidate,omitempty"`
}