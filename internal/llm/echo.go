package llm

import "context"

// Echo replies without a model. Used until OPENAI/Gemini keys are wired.
type Echo struct{}

func (Echo) Generate(_ context.Context, _, user string) (string, error) {
	return "I received your message: " + user, nil
}
