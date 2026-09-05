package google

import "context"

// Provider bundles the exchange and the verification into the single dependency
// the auth service takes. They are separate types because they fail for
// unrelated reasons and are tested against different fakes, but no caller ever
// wants one without the other.
type Provider struct {
	exchanger *Exchanger
	verifier  *Verifier
}

func NewProvider(exchanger *Exchanger, verifier *Verifier) *Provider {
	return &Provider{exchanger: exchanger, verifier: verifier}
}

func (p *Provider) Exchange(ctx context.Context, code, codeVerifier, redirectURI string) (string, error) {
	return p.exchanger.Exchange(ctx, code, codeVerifier, redirectURI)
}

func (p *Provider) Verify(ctx context.Context, rawIDToken string) (Identity, error) {
	return p.verifier.Verify(ctx, rawIDToken)
}
