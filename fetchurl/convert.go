package fetchurl

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
)

type MarkdownHeader struct {
	TargetURL  string
	CurrentURL string
	Title      string
}

func (w *WebFetcher) WebpageToMarkdownYaml(webpage *FetchedWebPage) (string, error) {
	return HtmlToMarkdownYaml(webpage.Src, map[string]string{
		"target_url":  webpage.TargetURL,
		"current_url": webpage.CurrentURL,
		"title":       webpage.Title,
	}, w.opts.UsePandoc)
}

func HtmlToMarkdownYaml(src string, headers map[string]string, usePandoc bool) (string, error) {
	var markdown string
	var err error

	if usePandoc {
		markdown, err = HtmlToMarkdownPandoc(src)
	} else {
		markdown, err = HtmlToMarkdown(src)
	}

	if err != nil {
		return "", err
	}

	ret := ""
	if len(headers) > 0 {
		ret += "---\n"
		for k, v := range headers {
			ret += fmt.Sprintf("%s: %s\n", k, v)
		}
		ret += "---\n"
	}

	ret += markdown
	return ret, nil
}

func HtmlToMarkdown(html string) (string, error) {
	return htmltomarkdown.ConvertString(html)
}

func HtmlToMarkdownPandoc(html string) (string, error) {
	ctx := context.Background()
	cmd := exec.CommandContext(
		ctx,
		"pandoc",
		"-f", "html",
		"-t", "gfm", // or "markdown", "markdown_strict", etc.
		"--wrap=preserve",
	)

	cmd.Stdin = strings.NewReader(html)

	// Capture stdout+stderr; if pandoc fails, you'll get the stderr in the error.
	out, err := cmd.CombinedOutput()
	ctx.Done()
	if err != nil {
		return "", fmt.Errorf("pandoc: %w: %s", err, out)
	}

	return string(out), nil
}
