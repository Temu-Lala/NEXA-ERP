package models

// Provider-aware dispatch. A governed connection targets one of several external systems, and the
// transport used to verify it differs: email is reached over SMTP (a raw guarded socket handshake),
// while webhook/REST/SMS/storage/other are all reached over HTTPS through the shared sender. These
// pure helpers choose the adapter and apply the matching endpoint policy, and they label the probe
// so the delivery log records how each attempt was actually made.

const (
	adapterHTTP = "http"
	adapterSMTP = "smtp"
)

// adapterForProvider maps a provider to its test transport.
func adapterForProvider(provider string) string {
	if ConnectionProvider(provider) == ProvEmail {
		return adapterSMTP
	}
	return adapterHTTP
}

// validateProviderEndpoint applies the transport-correct pure policy to a configured endpoint:
// an absolute HTTPS URL for HTTP adapters, a host[:port] mail target for SMTP.
func validateProviderEndpoint(provider, endpoint string) error {
	if adapterForProvider(provider) == adapterSMTP {
		_, err := parseSMTPTarget(endpoint)
		return err
	}
	return validateEndpointPolicy(endpoint)
}

// probeMethodLabel is the value stored in the delivery row's method column for the transport used.
func probeMethodLabel(provider, httpMethod string) string {
	if adapterForProvider(provider) == adapterSMTP {
		return "SMTP"
	}
	return toUpperMethod(httpMethod)
}
