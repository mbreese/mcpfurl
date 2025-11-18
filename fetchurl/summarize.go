package fetchurl

import (
	"context"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type WebPageSummary struct {
	TargetURL  string `json:"target_url"`
	CurrentURL string `json:"current_url"`
	Title      string `json:"title"`
	Summary    string `json:"summary"`
}

func (s WebPageSummary) ToYaml() string {
	return fmt.Sprintf(`---
target_url: %s
current_url: %s
title: %s
---
%s
`, s.TargetURL, s.CurrentURL, s.Title, s.Summary)
}

func (w *WebFetcher) SummarizeURL(ctx context.Context, targetURL string, selector string, short bool) (*WebPageSummary, error) {
	webpage, err := w.FetchURL(ctx, targetURL, selector)
	if err != nil {
		return nil, err
	}

	w.opts.Logger.Debug(fmt.Sprintf("Loaded URL: %s", targetURL))
	md, err := w.WebpageToMarkdownYaml(webpage)
	if err != nil {
		return nil, err
	}

	client := openai.NewClient(
		option.WithAPIKey(w.opts.SummarizeApiKey),
		option.WithBaseURL(w.opts.SummarizeBaseURL),
	)
	w.opts.Logger.Debug(fmt.Sprintf("Sending to LLM: %s [%s]", w.opts.SummarizeBaseURL, w.opts.SummarizeModel))

	var msg string
	if w.opts.SummarizeShort || short {
		msg = "Using only 1-3 sentences, summarize the document below:\n\n<DOCUMENT>\n" + md
	} else {
		msg = "Summarize the document below:\n\n<DOCUMENT>\n" + md
	}

	chatCompletion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(msg),
		},
		Model: w.opts.SummarizeModel,
	})
	if err != nil {
		return nil, err
	}
	summary := chatCompletion.Choices[0].Message.Content

	return &WebPageSummary{
		TargetURL:  webpage.TargetURL,
		CurrentURL: webpage.CurrentURL,
		Title:      webpage.Title,
		Summary:    summary,
	}, err
}
