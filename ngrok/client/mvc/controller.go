package mvc

import "ngrok-plus/ngrok/util"

type Controller interface {
	// Update how the model communicates that it has changed state
	Update(State)

	// Shutdown instructs the controller to shut the app down
	Shutdown(message string)

	// PlayRequest instructs the model to play requests
	PlayRequest(tunnel Tunnel, payload []byte)

	// Updates A channel of updates
	Updates() *util.Broadcast

	// State returns the current state
	State() State

	// Go safe wrapper for running go-routines
	Go(fn func())

	// GetWebInspectAddr the address where the web inspection interface is running
	GetWebInspectAddr() string
}
