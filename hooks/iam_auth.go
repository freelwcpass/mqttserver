package hooks

import (
	"context"
	"log"
	"time"

	"github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"github.com/freelwcpass/iot-core/iam"
)

// IAMAuthHook integrates with the IAM service for MQTT client authentication and authorization.
type IAMAuthHook struct {
	mqtt.HookBase
	iamClient iam.IAMClient
	conn      *grpc.ClientConn
}

// NewIAMAuthHook creates a new authentication hook with a gRPC connection to the IAM service.
func NewIAMAuthHook(iamAddr string) (*IAMAuthHook, error) {
	// Establish gRPC connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		iamAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()), // Use TLS in production
		grpc.WithBlock(),
	)
	if err != nil {
		log.Printf("Failed to connect to IAM service at %s: %v", iamAddr, err)
		return nil, err
	}

	client := iam.NewIAMClient(conn)
	return &IAMAuthHook{
		iamClient: client,
		conn:      conn,
	}, nil
}

// ID returns the hook identifier.
func (h *IAMAuthHook) ID() string {
	return "iam-auth-hook"
}

// Provides indicates the hook capabilities.
func (h *IAMAuthHook) Provides(b byte) bool {
	return b == mqtt.OnConnectAuthenticate || b == mqtt.OnACLCheck
}

// Init initializes the hook (placeholder for future configuration).
func (h *IAMAuthHook) Init(config any) error {
	log.Println("IAMAuthHook initialized")
	return nil
}

// Stop closes the gRPC connection.
func (h *IAMAuthHook) Stop() error {
	log.Println("Closing IAMAuthHook gRPC connection")
	return h.conn.Close()
}

// OnConnectAuthenticate authenticates MQTT clients using the IAM service.
func (h *IAMAuthHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := &iam.AuthenticateRequest{
		ClientId: string(cl.ID),
		Token:    string(pk.Connect.Password),
	}

	resp, err := h.iamClient.Authenticate(ctx, req)
	if err != nil {
		log.Printf("Authentication failed for client %s: %v", cl.ID, err)
		return false
	}

	if !resp.Success {
		log.Printf("Authentication denied for client %s: %s", cl.ID, resp.Error)
		return false
	}

	log.Printf("Client %s authenticated successfully", cl.ID)
	return true
}

// OnACLCheck checks topic access permissions using the IAM service.
func (h *IAMAuthHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := &iam.AuthorizeRequest{
		ClientId: string(cl.ID),
		Topic:    topic,
		Write:    write,
	}

	resp, err := h.iamClient.Authorize(ctx, req)
	if err != nil {
		log.Printf("Authorization failed for client %s on topic %s: %v", cl.ID, topic, err)
		return false
	}

	if !resp.Allowed {
		log.Printf("Authorization denied for client %s on topic %s: %s", cl.ID, topic, resp.Error)
		return false
	}

	log.Printf("Client %s authorized for topic %s (write: %v)", cl.ID, topic, write)
	return true
}

// OnAuthPacket processes authentication packets (not used in this implementation).
func (h *IAMAuthHook) OnAuthPacket(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	// Return the packet unchanged, as we handle authentication in OnConnectAuthenticate
	return pk, nil
}
