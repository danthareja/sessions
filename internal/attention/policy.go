package attention

type Policy struct {
	states map[string]struct{}
}

func New(states []string) Policy {
	p := Policy{states: map[string]struct{}{}}
	for _, state := range states {
		if state == "" {
			continue
		}
		p.states[state] = struct{}{}
	}
	return p
}

func (p Policy) Attention(state string) bool {
	_, ok := p.states[state]
	return ok
}
