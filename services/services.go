package services

// SendWhatsAppMessage is a function pointer that will be set by main.go
// to avoid import cycle between services and handlers
var SendWhatsAppMessage func(recipient string, message string, mediaPath string, options ...string) error
