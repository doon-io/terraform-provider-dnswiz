package client

import (
	"context"
	"net/http"
	"time"
)

// IssueRequest is the payload for POST /v1/certs/issue.
//
// Exactly one of CSR or Names must be set:
//
//   - Names: server generates an ECDSA P-256 keypair, signs a CSR, runs
//     ACME, and returns the private key in the response. The key is
//     never persisted server-side; the response is the only copy.
//   - CSR: caller-supplied PEM. Server signs whatever SANs are in it.
//     The caller already has the key.
type IssueRequest struct {
	Names []string `json:"names,omitempty"`
	CSR   string   `json:"csr,omitempty"`
}

// Issuance is the response from POST /v1/certs/issue. KeyPEM is
// populated only when the request used Names (managed-keypair mode).
type Issuance struct {
	KeyPEM       string    `json:"key_pem,omitempty"`
	CertPEM      string    `json:"cert_pem"`
	IssuerPEM    string    `json:"issuer_pem"`
	FullChainPEM string    `json:"fullchain_pem"`
	Serial       string    `json:"serial"`
	ExpiresAt    time.Time `json:"expires_at"`
	SANs         []string  `json:"sans"`
}

// IssueCert opens an ACME order and blocks until the cert is signed.
// ACME flows can take 10–60s depending on propagation; callers should
// configure a generous HTTP timeout. The default client.Client timeout
// (30s) is too short — IssueCert overrides it to 180s for this call.
func (c *Client) IssueCert(ctx context.Context, req IssueRequest) (*Issuance, error) {
	// Stash + restore the global timeout so the IssueCert call doesn't
	// permanently raise the bar for other calls sharing this Client.
	prev := c.httpClient.Timeout
	c.httpClient.Timeout = 180 * time.Second
	defer func() { c.httpClient.Timeout = prev }()

	var out Issuance
	if err := c.Do(ctx, http.MethodPost, "/v1/certs/issue", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
