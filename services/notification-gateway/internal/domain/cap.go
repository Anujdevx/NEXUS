// Package domain builds public alerts: CAP-style (Common Alerting Protocol) messages in English and Hindi.
package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Default public text. Source: frontend/src/i18n.js, ALERT_TEXT (the console's own wording).
const (
	DefaultEN = "River rising. Move away from the bank to higher ground now."
	DefaultHI = "नदी का जलस्तर बढ़ रहा है। किनारे से हटकर तुरंत ऊँचे स्थान पर जाएँ।"
)

var Severities = map[string]bool{"Extreme": true, "Severe": true, "Moderate": true, "Minor": true, "Unknown": true}

type Alert struct {
	ID       string         `json:"id"`
	Area     string         `json:"area"`
	Severity string         `json:"severity"`
	TextEN   string         `json:"text_en"`
	TextHI   string         `json:"text_hi"`
	IssuedAt time.Time      `json:"issued_at"`
	IssuedBy string         `json:"issued_by"`
	Source   string         `json:"source"`
	CAP      map[string]any `json:"cap"`
}

// NewAlert fills defaults and builds the CAP document.
func NewAlert(area, severity, en, hi, by, source, event string) (Alert, error) {
	if strings.TrimSpace(area) == "" {
		return Alert{}, fmt.Errorf("area is required")
	}
	if severity == "" {
		severity = "Severe"
	}
	if !Severities[severity] {
		return Alert{}, fmt.Errorf("severity must be Extreme, Severe, Moderate, Minor or Unknown")
	}
	if strings.TrimSpace(en) == "" && strings.TrimSpace(hi) == "" {
		en, hi = DefaultEN, DefaultHI
	}
	if event == "" {
		event = "Flood"
	}
	a := Alert{ID: "NEXUS-DDN-" + uuid.NewString()[:8], Area: area, Severity: severity, TextEN: en, TextHI: hi, IssuedAt: time.Now().UTC(), IssuedBy: by, Source: source}
	a.CAP = a.buildCAP(event)
	return a, nil
}

func (a Alert) buildCAP(event string) map[string]any {
	info := func(lang, text string) map[string]any {
		return map[string]any{"language": lang, "category": "Met", "event": event, "urgency": "Immediate", "severity": a.Severity, "certainty": "Likely",
			"headline": fmt.Sprintf("%s: %s", a.Severity, a.Area), "description": text, "area": map[string]string{"areaDesc": a.Area}}
	}
	infos := []map[string]any{}
	if a.TextEN != "" {
		infos = append(infos, info("en-IN", a.TextEN))
	}
	if a.TextHI != "" {
		infos = append(infos, info("hi-IN", a.TextHI))
	}
	return map[string]any{"identifier": a.ID, "sender": "sutradhara@nexus", "sent": a.IssuedAt.Format(time.RFC3339), "status": "Exercise",
		"msgType": "Alert", "scope": "Public", "info": infos, "note": "Demo alert. Nothing is sent to the public."}
}

// Template texts for automatic alerts, by failed utility type.
func InfraText(assetType, area string) (en, hi string) {
	switch assetType {
	case "POWER":
		return fmt.Sprintf("Power supply failure near %s. Some hospitals may divert patients. Go to the nearest hospital that is open.", area),
			fmt.Sprintf("%s के पास बिजली आपूर्ति ठप है। कुछ अस्पताल मरीजों को दूसरे अस्पताल भेज सकते हैं। निकटतम खुले अस्पताल जाएँ।", area)
	case "TELECOM":
		return fmt.Sprintf("Mobile tower failure near %s. Use SMS or the SOS app; requests will be relayed.", area),
			fmt.Sprintf("%s के पास मोबाइल टावर बंद है। SMS या SOS ऐप का उपयोग करें; अनुरोध आगे भेजे जाएँगे।", area)
	case "WATER":
		return fmt.Sprintf("Water pump failure near %s. Store drinking water and boil it before use.", area),
			fmt.Sprintf("%s के पास जल पंप बंद है। पीने का पानी जमा करें और उबालकर पिएँ।", area)
	}
	return fmt.Sprintf("Infrastructure failure near %s.", area), fmt.Sprintf("%s के पास अवसंरचना में खराबी है।", area)
}
