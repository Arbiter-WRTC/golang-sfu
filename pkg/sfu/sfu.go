package SFU

import ( 
	"fmt"
	"time"
	"sync"
	"github.com/gorilla/websocket" 
	"log"
	"encoding/json"
	"arbiter-go-sfu/pkg/client"
	"arbiter-go-sfu/pkg/arbiterTypes"
	"github.com/pion/webrtc/v3"
)

type SFU struct {
	sync.RWMutex
	clients 	    map[string]*Client.Client
	socket          *websocket.Conn
	eventEmitter    map[string]chan string
	sfuId 		    string
	rtcConfig       []webrtc.ICEServer
}

type IdentifyMessage struct {
    Action string `json:"action"`
    Data   IdentifyPayload `json:"data"`	
}

type IdentifyPayload struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}


func NewSFU(socketUrl, sfuId string, rtcConfig []webrtc.ICEServer) *SFU {
	socket, err := connectWebSocket(socketUrl, sfuId)
	if err != nil {
		log.Fatal(err)
		// return
	}

	eventEmitter := make(map[string] chan string)
	eventEmitter["featuresShared"] = make(chan string)
	eventEmitter["producerTrack"] = make(chan string)
	
	sfu := &SFU{
		clients: make(map[string]*Client.Client),
		socket: socket,
		eventEmitter: eventEmitter,
		sfuId: sfuId,
		rtcConfig: rtcConfig,
	}
	
	go sfu.listenForEvents()
	// identify sends a signal to the websocket server that tells it who it is, so it can be stored in the db
	sfu.identify()
	// starts a goroutine that loops listening for signals from the websocket server
	go sfu.listenOnSocket()
	return sfu
}

func connectWebSocket(socketUrl string, sfuId string) (*websocket.Conn, error) {
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(socketUrl, nil)
	if err != nil {
			log.Fatal(err)
			return nil, err
	}

	log.Println("Connected to WebSocket server:", socketUrl)
	
	return conn, nil
}

func (sfu *SFU) identify() {
	payload := IdentifyMessage {
        Action: "identify",
        Data: IdentifyPayload {
            ID:   sfu.sfuId,
            Type: "sfu",
        },
    }

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to serialize JSON payload: %v\n", err)
		return
	}

	err = sfu.socket.WriteMessage(websocket.TextMessage, payloadJSON)
	if err != nil {
		log.Printf("Failed to send payload over WebSocket: %v\n", err)
		return
	}
}

func (sfu *SFU) listenOnSocket() {
	var maxMessageSize int64 = 1024
	pongWait := 60 * time.Second

	defer func() {
		sfu.socket.Close()
	}()

	sfu.socket.SetReadLimit(maxMessageSize)
	sfu.socket.SetReadDeadline(time.Now().Add(pongWait))
	sfu.socket.SetPongHandler(func(string) error { sfu.socket.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := sfu.socket.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		sfu.handleMessage(message)
	}
}

func (sfu *SFU) listenForEvents() {
	for {
		select {
		case msg := <-sfu.eventEmitter["featuresShared"]:
				fmt.Println(msg)
		case msg := <-sfu.eventEmitter["producerTrack"]:
				fmt.Println(msg)
		}
	}
}


func (sfu *SFU) handleMessage(data []byte) {
	var parsedData ArbiterTypes.HandshakePayload
	log.Println("unparsed: ", string(data))
	// check if its a sdp or an ice candidate
	// if it's an ice candidate, we need to marshal it better
	if err := json.Unmarshal(data, &parsedData); err != nil {
		log.Printf("Failed to parse JSON payload: %v\n", err)
		return
	}

	log.Println("parsed: ", parsedData)

	if parsedData.Type == "producer" {
		// handle producer
		log.Println("Handing producer message")
		sfu.handleProducerHandshake(parsedData)
	} else if parsedData.Type == "consumer" {
		// handle consumer
	}
}

// HandleProducerHandshake
// 	Get the sender attribute
//  Find client by sender attribute
//  If Client doesn't exist
//     Create client
//  Call ProducerHandshake on Client
func (sfu *SFU) handleProducerHandshake(data ArbiterTypes.HandshakePayload) {
	sender := data.Sender
	client := sfu.findClientById(sender)

	if client == nil {
		// create a new client
		log.Println("Adding a new client")
		client = sfu.addClient(sender)
	}
	client.ProducerHandshake(data);
}

func (sfu *SFU) findClientById(clientId string) *Client.Client {
	log.Println("the id is:", clientId)
	client, ok := sfu.clients[clientId]
	if ok {
		log.Println("Found client, returning")
		return client
	}
	log.Println("No client found, returning nil")
	return nil
}

func (sfu *SFU) addClient(clientId string) *Client.Client {
	client := Client.NewClient(clientId, sfu.rtcConfig, sfu.socket,sfu.eventEmitter)
	sfu.clients[clientId] = client
	return client
}

// func (sfu *SFU) findClientById()


// func handleConnect(sfuId string) {
// 	log.Println("Connected to WebSocket Server:")

// 	payload := IdentifyPayload{
// 		Action: "identify",
// 		Data: struct {
// 			ID   string `json:"id"`
// 			Type string `json:"type`
// 		}{
// 			ID: sfuId,
// 			Type: "sfu",
// 		},
// 	}

// 	payloadJSON, err := json.Marshal(payload)
// 	if err != nil {
// 		log.Printf("Failed to serialize JSON payload: %v\n", err)
// 		return
// 	}

// 	err = sfu.socket.WriteMessage(websocket.TextMessage, payloadJSON)
// 	if err != nil {
// 		log.Printf("Failed to send payload over WebSocket: %v\n", err)
// 		return
// 	}

// 	log.Printf("Identifying... %s\n", sfuId)
// }

// // These methods would need proper implementation in Go
// func (sfu *SFU) bindClientEvents() {
// 	// ... Bind client events ...
// }

// func (sfu *SFU) handleFeaturesShared(id string, features interface{}, initialConnect bool) {
// 	// ... Handle shared features ...
// }

// func (sfu *SFU) featuresCatchup(catchupClientId string) {
// 	// ... Implement features catchup logic ...
// }

// func (sfu *SFU) handleProducerTrack(id string, track interface{}) {
// 	// ... Handle producer track ...
// }

// func (sfu *SFU) consumerCatchup(catchupClientId string) {
// 	// ... Implement consumer catchup logic ...
// }

// func (sfu *SFU) bindSocketEvents() {
// 	// ... Bind socket events ...
// }

// func (sfu *SFU) handleConnect() {
// 	fmt.Println("Connected to websocket server")
// 	// sfu.socket.emit would emit events on the socket
// }

// func (sfu *SFU) handleProducerHandshake(clientId string, description interface{}, candidate interface{}) {
// 	// ... Handle producer handshake ...
// }

// func (sfu *SFU) handleConsumerHandshake(clientId string, remotePeerId string, description interface{}, candidate interface{}) {
// 	// ... Handle consumer handshake ...
// }

// func (sfu *SFU) handleClientDisconnect(clientId string) {
// 	// ... Handle client disconnect ...
// }

// func (sfu *SFU) addClient(id string) *Client {
// 	// ... Add a new client ...
// 	return &Client{} // Return a reference to the new client
// }

// func (sfu *SFU) findClientById(id string) *Client {
// 	// ... Find client by ID ...
// 	return &Client{} // Return a reference to the found client
// }