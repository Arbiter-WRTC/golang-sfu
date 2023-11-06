package Consumer

import (
	"log"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
	"arbiter-go-sfu/pkg/arbiterTypes"
	"encoding/json"
	"sync"
)

type Consumer struct {
	sync.RWMutex
	clientId 			 		string
	sfuId						  string
	remotePeerId			string
	socket					  *ArbiterTypes.SafeConnection
	connection 			  *webrtc.PeerConnection
	rtcConfig 				[]webrtc.ICEServer
	mediaTracks				map[string] *webrtc.TrackRemote
	candidates 			  []webrtc.ICECandidateInit
}

type ConsumerDescriptionMessage struct {
    Action string `json:"action"`
    Data   ConsumerDescriptionPayload `json:"data"`	
}

type ConsumerDescriptionPayload struct {
	Type 				string 										 `json:"type"`
	Sender 			string 										 `json:"sender"`
	Receiver 		string 										 `json:"receiver"`
	RemotePeer  string										 `json:"remotePeerId"`
	Description *webrtc.SessionDescription `json:"description"`
}

type IceMessage struct {
    Action string `json:"action"`
    Data   ConsumerIcePayload `json:"data"`	
}

type ConsumerIcePayload struct {
	Type 				string 										 `json:"type"`
	Sender 			string 										 `json:"sender"`
	RemotePeer  string										 `json:"remotePeer`
	Receiver 		string 										 `json:"receiver"`
	Candidate 	*webrtc.ICECandidate 			 `json:"candidate"`
}

// TODO: PASS IN SFU ID
func NewConsumer(clientId,
				 sfuId,
				 remotePeerId string,
				 rtcConfig []webrtc.ICEServer,
				 socket *ArbiterTypes.SafeConnection,
				) *Consumer {

	config := webrtc.Configuration{ICEServers: rtcConfig}
	connection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		log.Println("Failed to establish RTCPeerConnection")
	}
	
	consumer := &Consumer{
		clientId: clientId,
		sfuId: sfuId,
		remotePeerId: remotePeerId,
		socket: socket,
		mediaTracks: make(map[string] *webrtc.TrackRemote),
		connection: connection,
	}
	consumer.registerConnectionCallbacks()
	return consumer
}

func (consumer *Consumer) registerConnectionCallbacks() {
	consumer.connection.OnICECandidate(consumer.handleIceCandidate)
	consumer.connection.OnNegotiationNeeded(consumer.handleRTCNegotiation)
}

func (consumer *Consumer) handleIceCandidate(candidate *webrtc.ICECandidate) {
	log.Println("Got an ICE candidate:", candidate);
	payload := IceMessage {
		Action: "handshake",
		Data: ConsumerIcePayload {
			Type: "consumer",
			Sender: consumer.sfuId, 
			Receiver: consumer.clientId,
			RemotePeer: consumer.remotePeerId,
			Candidate: candidate,
		},
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to serialize JSON payload: %v\n", err)
		return
	}
	consumer.socket.Mu.Lock()
	defer consumer.socket.Mu.Unlock()
	err = consumer.socket.Conn.WriteMessage(websocket.TextMessage, payloadJSON)
	if err != nil {
		log.Printf("Failed to send payload over WebSocket: %v\n", err)
		return
	}
}

func (consumer *Consumer) handleRTCNegotiation() {
	log.Println("consumer attempting offer")
	options := webrtc.OfferAnswerOptions{
		VoiceActivityDetection: false,
	}

	offerOptions := &webrtc.OfferOptions{OfferAnswerOptions: options, ICERestart: false}
	sdp, err := consumer.connection.CreateOffer(offerOptions)
	if err != nil {
		log.Println("Error creating offer:", err)
			return
	}
	consumer.connection.SetLocalDescription(sdp)

	payload := ConsumerDescriptionMessage {
		Action: "handshake",
		Data: ConsumerDescriptionPayload {
			Type: "consumer",
			Sender: consumer.sfuId, 
			Receiver: consumer.clientId,
			RemotePeer: consumer.remotePeerId,
			Description: consumer.connection.LocalDescription(),
		},
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to serialize JSON payload: %v\n", err)
		return
	}

	consumer.socket.Mu.Lock()
	defer consumer.socket.Mu.Unlock()
	err = consumer.socket.Conn.WriteMessage(websocket.TextMessage, payloadJSON)
	if err != nil {
		log.Printf("Failed to send payload over WebSocket: %v\n", err)
		return
	}
}

func (consumer *Consumer) AddTrack(remoteTrack *webrtc.TrackRemote) {
	localTrack, err := webrtc.NewTrackLocalStaticRTP(remoteTrack.Codec().RTPCodecCapability, remoteTrack.ID(), remoteTrack.StreamID())
	if err != nil {
		log.Println("error creating local track", err)
	}
	
	_, trackErr := consumer.connection.AddTrack(localTrack)
	if trackErr != nil {
		log.Println("error adding local track to consumer", err)
	}
	
	go forwardPackets(remoteTrack, localTrack)	
}

func forwardPackets(remoteTrack *webrtc.TrackRemote, consumerTrack *webrtc.TrackLocalStaticRTP) {
	for {
		rtpPacket, _, readErr := remoteTrack.ReadRTP()
		if readErr != nil {
			log.Println("error reading producer track", readErr)
		}

		writeErr := consumerTrack.WriteRTP(rtpPacket)
		if writeErr != nil {
			log.Println("error writing to producer track", writeErr)
		}
	}
}

func (consumer *Consumer) Handshake(data ArbiterTypes.HandshakePayload) {
	if data.Description.Type.String() == "answer" {
		err := consumer.connection.SetRemoteDescription(data.Description)
		if err != nil {
			log.Println("Error setting remote description:", err)
			return
		}
		consumer.processIceCandidates()
	} else {
		consumer.handleReceivedIceCandidate(data.Candidate)
	}
}

func (consumer *Consumer) processIceCandidates() {
	for _, candidate := range consumer.candidates {
		err := consumer.connection.AddICECandidate(candidate)
		if err != nil {
			log.Println("Error adding ICE Candidate:", err)
			return
		}
	}
}

func (consumer *Consumer) handleReceivedIceCandidate(candidate webrtc.ICECandidateInit) {
	if (consumer.connection.RemoteDescription() == nil) {
		consumer.candidates = append(consumer.candidates, candidate)
	} else {
		err := consumer.connection.AddICECandidate(candidate)
		if err != nil {
			log.Println("Error adding ICE Candidate:", err)
			return
		}
	}
}

func (consumer *Consumer) CloseConnection() {
	err := consumer.connection.Close()
	if err != nil {
		log.Println("Error closing connection:", err)
	}
}