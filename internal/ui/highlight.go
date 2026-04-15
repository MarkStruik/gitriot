package ui

import (
	"bytes"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

func HighlightForPath(path string, content string) string {
	lexer := lexers.Match(path)
	if lexer == nil {
		lexer = lexers.Analyse(content)
	}
	if lexer == nil {
		return content
	}

	lexer = chroma.Coalesce(lexer)
	iterator, err := lexer.Tokenise(nil, content)
	if err != nil {
		return content
	}

	formatter := formatters.Get("terminal256")
	if formatter == nil {
		return content
	}

	style := styles.Get("github-dark")
	if style == nil {
		style = styles.Fallback
	}

	var out bytes.Buffer
	if err := formatter.Format(&out, style, iterator); err != nil {
		return content
	}

	return out.String()
}
