package Producer

import (
	"log"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
	"arbiter-go-sfu/pkg/arbiterTypes"
	"encoding/json"
	"sync"
)

type Producer struct {
	sync.RWMutex
	clientId 			 		string
	sfuId						  string
	socket					  *ArbiterTypes.SafeConnection
	producerTrackChannel chan ArbiterTypes.ProducerTrackChannel
	featuresSharedChannel chan string
	isNegotiating		 	bool
	mediaTracks				map[string] *webrtc.TrackRemote
	connection 			  *webrtc.PeerConnection
	candidates 			  []webrtc.ICECandidateInit
}

type DescriptionMessage struct {
    Action string `json:"action"`
    Data   DescriptionPayload `json:"data"`	
}

type DescriptionPayload struct {
	Type 				string 										 `json:"type"`
	Sender 			string 										 `json:"sender"`
	Receiver 		string 										 `json:"receiver"`
	Description *webrtc.SessionDescription `json:"description"`
}

type IceMessage struct {
    Action string `json:"action"`
    Data   IcePayload `json:"data"`	
}

type IcePayload struct {
	Type 				string 										 `json:"type"`
	Sender 			string 										 `json:"sender"`
	Receiver 		string 										 `json:"receiver"`
	Candidate 	*webrtc.ICECandidate 			 `json:"candidate"`
}

// TODO: PASS IN SFU ID
func NewProducer(clientId,
				 sfuId string,
				 rtcConfig []webrtc.ICEServer,
				 socket *ArbiterTypes.SafeConnection,
				 producerTrackChannel chan ArbiterTypes.ProducerTrackChannel,
				 featuresSharedChannel chan string) *Producer {

	config := webrtc.Configuration{ICEServers: rtcConfig}
	connection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		log.Println("Failed to establish RTCPeerConnection")
	}
	
	producer := &Producer{
		clientId: clientId,
		sfuId: sfuId,
		socket: socket,
		producerTrackChannel: producerTrackChannel,
		featuresSharedChannel: featuresSharedChannel,
		mediaTracks: make(map[string] *webrtc.TrackRemote),
		connection: connection,
	}
	producer.registerConnectionCallbacks()

	return producer
}

func (producer *Producer) registerConnectionCallbacks() {
	producer.connection.OnICECandidate(producer.handleIceCandidate)
	producer.connection.OnTrack(producer.handleOnTrack)
}

func (producer *Producer) handleIceCandidate(candidate *webrtc.ICECandidate) {
	payload := IceMessage {
		Action: "handshake",
		Data: IcePayload {
			Type: "producer",
			Sender: producer.sfuId, 
			Receiver: producer.clientId,
			Candidate: candidate,
		},
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to serialize JSON payload: %v\n", err)
		return
	}

	producer.socket.Mu.Lock()
	defer producer.socket.Mu.Unlock()
	err = producer.socket.Conn.WriteMessage(websocket.TextMessage, payloadJSON)
	if err != nil {
		log.Printf("Failed to send payload over WebSocket: %v\n", err)
		return
	}
}

func (producer *Producer) handleOnTrack(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	log.Println(`handle incoming %s track`, track.Kind())
	producer.mediaTracks[string(track.Kind())] = track
	trackMessage := ArbiterTypes.ProducerTrackChannel{producer.clientId, track}
	producer.producerTrackChannel <- trackMessage
}

func (producer *Producer) GetMediaTracks() map[string] *webrtc.TrackRemote {
	return producer.mediaTracks
}

func (producer *Producer) Handshake(data ArbiterTypes.HandshakePayload) {
	if data.Description.Type.String() == "offer" {
		err := producer.connection.SetRemoteDescription(data.Description)
		if err != nil {
			log.Println("Error setting remote description:", err)
			return
		}
		log.Println("Set remote description")
		options := webrtc.OfferAnswerOptions{
			VoiceActivityDetection: false,
		}

		answerOpts := webrtc.AnswerOptions{OfferAnswerOptions: options}
		answer, err := producer.connection.CreateAnswer(&answerOpts)

		if err != nil {
			log.Println("Error creating offer:", err)
			return
		}

		answerErr := producer.connection.SetLocalDescription(answer)
		if answerErr != nil {
			log.Println("Error setting local description:", answerErr)
			return
		}

		payload := DescriptionMessage {
			Action: "handshake",
			Data: DescriptionPayload {
				Type: "producer",
				Sender: producer.sfuId, 
				Receiver: producer.clientId,
				Description: producer.connection.LocalDescription(),
			},
		}

		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			log.Printf("Failed to serialize JSON payload: %v\n", err)
			return
		}

		producer.socket.Mu.Lock()
		defer producer.socket.Mu.Unlock()
		err = producer.socket.Conn.WriteMessage(websocket.TextMessage, payloadJSON)
		if err != nil {
			log.Printf("Failed to send payload over WebSocket: %v\n", err)
			return
		}
		producer.processIceCandidates()
	} else {
		producer.handleReceivedIceCandidate(data.Candidate)
	}
	// if data.Description != nil {
	// } else if data.Candidate != nil {
	// }
}

func (producer *Producer) handleReceivedIceCandidate(candidate webrtc.ICECandidateInit) {
	if (producer.connection.RemoteDescription() == nil) {
		producer.candidates = append(producer.candidates, candidate)
	} else {
		err := producer.connection.AddICECandidate(candidate)
		if err != nil {
			log.Println("Error adding ICE Candidate:", err)
			return
		}
	}
}

func (producer *Producer) processIceCandidates() {
	for _, candidate := range producer.candidates {
		err := producer.connection.AddICECandidate(candidate)
		if err != nil {
			log.Println("Error adding ICE Candidate:", err)
			return
		}
	}
}

func (producer *Producer) AddChatChannel() {
	negotiated := true
	options := webrtc.DataChannelInit{
		Negotiated: &negotiated,
	}
	
	_, err := producer.connection.CreateDataChannel("chat", &options)
	if err != nil {
		log.Println("Error creating chat channel")
		return
	}

	producer.connection.OnDataChannel(producer.handleDataChannel)
}

func (producer *Producer) handleDataChannel(dataChannel *webrtc.DataChannel) {
	log.Println(dataChannel) 
}

func (producer *Producer) AddFeaturesChannel() {
	
}

func (producer *Producer) ShareFeatures() {
	
}

func (producer *Producer) SetFeatures() {
	
}

func (producer *Producer) GetFeatures() {
	
}

func (producer *Producer) CloseConnection() {
	err := producer.connection.Close()
	if err != nil {
		log.Println("Error closing connection:", err)
	}
}