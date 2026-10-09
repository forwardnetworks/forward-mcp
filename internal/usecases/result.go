package usecases

// Result is what a tool handler produces: one block of text for the model.
type Result struct {
	Text string
}

// textResult builds a tool result from text.
func textResult(text string) *Result {
	return &Result{Text: text}
}
