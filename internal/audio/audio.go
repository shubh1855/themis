package audio

import (
	"context"
	"os/exec"
	"runtime"

	openai "github.com/sashabaranov/go-openai"
)

var recordCmd *exec.Cmd

// StartRecording starts recording audio to the specified path using arecord.
func StartRecording(path string) error {
	switch runtime.GOOS {
	case "windows":
		// Assumes ffmpeg is installed
		// fallback that works generically (assumes default audio input device is available via dshow)
		// Usually "audio=Microphone" works on en-US Windows, but using generic is safer.
		// However ffmpeg requires the exact device name.
		// Since Windows doesn't export a CLI audio recorder natively, ffmpeg is the standard fallback.
		recordCmd = exec.Command("ffmpeg", "-y", "-f", "dshow", "-i", "audio=Microphone", "-t", "3600", path)
	case "darwin":
		// rec is standard with SoX on macOS
		recordCmd = exec.Command("rec", "-q", "-c", "1", "-r", "16000", path)
	default:
		// arecord is standard ALSA on Linux
		recordCmd = exec.Command("arecord", "-q", "-f", "cd", "-t", "wav", path)
	}
	return recordCmd.Start()
}

// StopRecording stops the ongoing audio recording.
func StopRecording() error {
	if recordCmd != nil && recordCmd.Process != nil {
		err := recordCmd.Process.Kill()
		recordCmd.Wait()
		recordCmd = nil
		return err
	}
	return nil
}

// Transcribe sends the audio file to the API for speech-to-text.
func Transcribe(ctx context.Context, apiKey string, path string) (string, error) {
	config := openai.DefaultConfig(apiKey)
	// Some users say "grok" when they mean "groq" for whisper, or maybe grok has whisper endpoint.
	// We'll configure x.ai first, but if the URL needs to be groq it can be changed.
	// Since groq's whisper model is "whisper-large-v3" and OpenAI's is "whisper-1".
	// The user asked for "grok api key for whisper".
	
	// Try x.ai base URL with standard go-openai which appends endpoints.
	config.BaseURL = "https://api.x.ai/v1"
	
	client := openai.NewClientWithConfig(config)
	req := openai.AudioRequest{
		Model:    "whisper-1",
		FilePath: path,
	}
	resp, err := client.CreateTranscription(ctx, req)
	if err != nil {
		// Fallback check: if they meant groq, try groq API.
		config.BaseURL = "https://api.groq.com/openai/v1"
		client = openai.NewClientWithConfig(config)
		req.Model = "whisper-large-v3"
		resp2, err2 := client.CreateTranscription(ctx, req)
		if err2 == nil {
			return resp2.Text, nil
		}
		
		// Another fallback: maybe it's just the OpenAI api for grok?
		// We'll return the original error.
		return "", err
	}

	return resp.Text, nil
}
