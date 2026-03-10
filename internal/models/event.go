package models

import "time"

// Event represents a single surveillance event triggered by a file upload.
type Event struct {
	ID          string    // Unique identifier for the event
	FilePath    string    // Temporary path to the uploaded file
	Timestamp   time.Time // When the file was received
	CameraIP    string    // IP address of the source camera
	Zone        string    // Zone the camera belongs to
	Labels      []string  // Tags associated with the event
	Description string    // Description of the event (free text)
}
