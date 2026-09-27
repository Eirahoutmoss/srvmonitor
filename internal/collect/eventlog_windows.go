//go:build windows

package collect

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"srvmon/internal/model"
)

// CollectEvents reads recent Critical (level 1) and Error (level 2) records
// from the System and Application logs via PowerShell's Get-WinEvent. Times are
// emitted as ISO 8601 by the query so parsing is stable across PowerShell
// versions.
func CollectEvents(window time.Duration, max int) []model.EventEntry {
	mins := int(window.Minutes())
	if mins < 1 {
		mins = 1
	}
	if max <= 0 {
		max = 50
	}
	script := fmt.Sprintf(`$ErrorActionPreference='SilentlyContinue'
$since=(Get-Date).AddMinutes(-%d)
Get-WinEvent -FilterHashtable @{LogName='System','Application';Level=1,2;StartTime=$since} -MaxEvents %d |
 Select-Object @{n='T';e={$_.TimeCreated.ToString('o')}}, `+
		`@{n='Id';e={[int64]$_.Id}}, @{n='Lvl';e={$_.LevelDisplayName}}, `+
		`@{n='Prov';e={$_.ProviderName}}, @{n='Log';e={$_.LogName}}, `+
		`@{n='Msg';e={($_.Message -replace '\s+',' ')}} |
 ConvertTo-Json -Depth 2 -Compress`, mins, max)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	return parseEvents(out)
}

type rawEvent struct {
	T    string `json:"T"`
	Id   int64  `json:"Id"`
	Lvl  string `json:"Lvl"`
	Prov string `json:"Prov"`
	Log  string `json:"Log"`
	Msg  string `json:"Msg"`
}

func parseEvents(b []byte) []model.EventEntry {
	b = []byte(strings.TrimSpace(string(b)))
	var raws []rawEvent
	if len(b) > 0 && b[0] == '[' {
		if json.Unmarshal(b, &raws) != nil {
			return nil
		}
	} else {
		var one rawEvent
		if json.Unmarshal(b, &one) != nil {
			return nil
		}
		raws = []rawEvent{one}
	}

	out := make([]model.EventEntry, 0, len(raws))
	for _, r := range raws {
		t, _ := time.Parse(time.RFC3339, r.T)
		msg := r.Msg
		if len(msg) > 300 {
			msg = msg[:300] + "…"
		}
		out = append(out, model.EventEntry{
			Time: t, Level: levelTR(r.Lvl), Log: r.Log,
			Provider: r.Prov, ID: r.Id, Message: msg,
		})
	}
	return out
}

func levelTR(l string) string {
	switch l {
	case "Critical":
		return "Kritik"
	case "Error":
		return "Hata"
	default:
		return l
	}
}
