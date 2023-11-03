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
	clientId string
	socket *websocket.Conn
	eventEmitter map[string]chan string
	isNegotiating bool
	connection *webrtc.PeerConnection
	candidates []webrtc.ICECandidateInit
	messagesReceived int
}

type DescriptionMessage struct {
    Action string `json:"action"`
    Data   DescriptionPayload `json:"data"`	
}

type DescriptionPayload struct {
	Type string `json:"type"`
	Sender string `json:"sender"`
	Receiver string `json:"receiver"`
	Description *webrtc.SessionDescription `json:"description"`
}

// TODO: PASS IN SFU ID
func NewProducer(clientId string, rtcConfig []webrtc.ICEServer, socket *websocket.Conn, eventEmitter map[string]chan string) *Producer {
	config := webrtc.Configuration{ICEServers: rtcConfig}
	connection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		log.Println("Failed to establish RTCPeerConnection")
	}
	
	producer := &Producer{
		clientId: clientId,
		socket: socket,
		eventEmitter: eventEmitter,
		connection: connection,
		messagesReceived: 0,
	}
	log.Println("created a new producer")
	return producer
}

func (producer *Producer) Handshake(data ArbiterTypes.HandshakePayload) {

	log.Printf("here we go payasos!!", data)
	producer.messagesReceived += 1
	log.Println("Messaged Received: ", producer.messagesReceived)
	if data.Description.Type.String() == "offer" {
		log.Println("Got an SDP")
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
				// TODO: remove hardcoded value
				Sender: "ABCD", 
				Receiver: producer.clientId,
				Description: producer.connection.LocalDescription(),
			},
		}

		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			log.Printf("Failed to serialize JSON payload: %v\n", err)
			return
		}

		err = producer.socket.WriteMessage(websocket.TextMessage, payloadJSON)
		if err != nil {
			log.Printf("Failed to send payload over WebSocket: %v\n", err)
			return
		}
		log.Println("i'm just chillin")
		producer.processIceCandidates()
	} else {
		log.Println("Got an ICE Candidate, handling")
		producer.handleReceivedIceCandidate(data.Candidate)
	}
	// if data.Description != nil {
	// } else if data.Candidate != nil {
	// }
}

func (producer *Producer) handleReceivedIceCandidate(candidate webrtc.ICECandidateInit) {
	if (producer.connection.RemoteDescription() == nil) {
		log.Println("Caching candidate")
		producer.candidates = append(producer.candidates, candidate)
	} else {
		log.Println("Adding an ice candidate")
		err := producer.connection.AddICECandidate(candidate)
		if err != nil {
			log.Println("Error adding ICE Candidate:", err)
			return
		}
		log.Println("Successfully added ICE Candidate")
	}
}

func (producer *Producer) processIceCandidates() {
	for _, candidate := range producer.candidates {
		log.Println("Adding an ice candidate")
		err := producer.connection.AddICECandidate(candidate)
		if err != nil {
			log.Println("Error adding ICE Candidate:", err)
			return
		}
		log.Println("Successfully added ICE Candidate")
	}
}

// async handshake(data) {
//     console.log(data);
//     const { description, candidate } = data;
//     if (description) {
//       console.log('trying to negotiate', description.type);

//       if (this.isNegotiating || this.connection.remoteDescription !== null) {
//         console.log('Skipping negotiation');
//         return;
//       }

//       this.isNegotiating = true;
//       // description.sdp = this.modifyIceAttributes(description.sdp);
//       await this.connection.setRemoteDescription(
//         decodeURIComponent(description)
//       );
//       this.isNegotiating = false;

//       if (description.type === 'offer') {
//         const answer = await this.connection.createAnswer();
//         await this.connection.setLocalDescription(answer);

//         console.log(
//           `Sending ${this.connection.localDescription.type} to ${this.clientId}`
//         );

//         const payload = {
//           action: 'handshake',
//           data: {
//             type: 'producer',
//             sender: this.sfuId,
//             receiver: this.clientId,
//             description: this.connection.localDescription,
//           },
//         };

//         console.log('Sending producer answer');
//         console.log('Receiver is:', this.clientId);

//         const json = JSON.stringify(payload);
//         this.socket.send(encodeURIComponent(json));
//         this.processQueuedCandidates();
//       }
//     } else if (candidate) {
//       try {
//         this.handleReceivedIceCandidate(decodeURIComponent(candidate));
//       } catch (e) {
//         if (candidate.candidate.length > 1) {
//           console.log('unable to add ICE candidate for peer', e);
//         }
//       }
//     }
//   }
