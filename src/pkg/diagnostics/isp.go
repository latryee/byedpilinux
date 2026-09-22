package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type ISPInfo struct {
	IP          string `json:"query"`
	ISP         string `json:"isp"`
	Org         string `json:"org"`
	AS          string `json:"as"`
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
}

type ISPDiagnosticResult struct {
	DetectedISP    string
	ASN            string
	Country        string
	Recommendation string
}

func DetectISP(ctx context.Context) (*ISPDiagnosticResult, error) {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "http://ip-api.com/json/?fields=query,isp,org,as,country,countryCode", nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to query ISP info (offline or blocked): %w", err)
	}
	defer resp.Body.Close()

	var info ISPInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	res := &ISPDiagnosticResult{
		DetectedISP: info.ISP,
		ASN:         info.AS,
		Country:     info.Country,
	}

	// Analyze ISP in Turkey
	asUpper := strings.ToUpper(info.AS)
	ispUpper := strings.ToUpper(info.ISP)

	if strings.Contains(asUpper, "AS9121") || strings.Contains(ispUpper, "TURK TELEKOM") || strings.Contains(ispUpper, "TTNET") {
		res.DetectedISP = "Türk Telekom (TTNet)"
		res.Recommendation = "Türk Telekom actively hijacks UDP 53 DNS and inspects SNI. Use Strategy C (TCP segmentation split2) with DoH enabled. Ensure 'Güvenli İnternet' is disabled in your TTNet portal."
	} else if strings.Contains(asUpper, "AS34984") || strings.Contains(ispUpper, "SUPERONLINE") || strings.Contains(ispUpper, "TURKCELL") {
		res.DetectedISP = "Turkcell Superonline"
		res.Recommendation = "Superonline utilizes Huawei DPI middleboxes injecting TCP RST upon seeing discord.com SNI. Strategy C (split2) or Strategy D (fake+split2) is highly effective. If voice RTC connects slowly, verify IPv4 preference is enabled."
	} else if strings.Contains(asUpper, "AS15897") || strings.Contains(ispUpper, "VODAFONE") {
		res.DetectedISP = "Vodafone Net"
		res.Recommendation = "Vodafone applies SNI filtering and DNS blocking. Strategy C with DoH is recommended."
	} else if strings.Contains(asUpper, "AS12735") || strings.Contains(ispUpper, "TURKNET") {
		res.DetectedISP = "TurkNet"
		res.Recommendation = "TurkNet applies BTK court-ordered blocks. Strategy C (split2) works reliably."
	} else if strings.Contains(asUpper, "AS47524") || strings.Contains(ispUpper, "TURKSAT") || strings.Contains(ispUpper, "KABLONET") {
		res.DetectedISP = "Türksat Kablonet"
		res.Recommendation = "Kablonet blocks Discord SNI on port 443. Strategy C (split2) works reliably."
	} else {
		if info.CountryCode == "TR" {
			res.Recommendation = fmt.Sprintf("Turkish ISP detected (%s). Standard Strategy C (TCP segmentation split2) with DoH is recommended.", info.ISP)
		} else {
			res.Recommendation = fmt.Sprintf("ISP: %s (%s). Strategy C (split2) is recommended if DPI filtering is active.", info.ISP, info.Country)
		}
	}

	return res, nil
}
