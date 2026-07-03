package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
)

const patSettingsURL = "https://app.bitrise.io/me/profile#/security"

func openBrowser(url string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", url)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		c = exec.Command("xdg-open", url)
	}
	return c.Start()
}

func openPATSettingsPage() {
	fmt.Println("Opening Bitrise Personal Access Token settings in your browser...")
	if err := openBrowser(patSettingsURL); err != nil {
		fmt.Printf("Could not open browser. Create a token at:\n  %s\n", patSettingsURL)
	}
}
