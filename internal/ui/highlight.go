package ui

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"gitriot/internal/theme"
)

func HighlightForPath(path string, content string, colors theme.Tokens) string {
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

	style := chroma.MustNewStyle("gitriot-theme", chroma.StyleEntries{
		chroma.Background:    "bg:" + colors.Bg,
		chroma.Text:          colors.SyntaxPlain,
		chroma.Keyword:       colors.SyntaxKeyword,
		chroma.KeywordType:   colors.SyntaxKeyword,
		chroma.NameFunction:  colors.SyntaxFunc,
		chroma.NameBuiltin:   colors.SyntaxFunc,
		chroma.NameClass:     colors.SyntaxType,
		chroma.LiteralString: colors.SyntaxString,
		chroma.LiteralNumber: colors.SyntaxNumber,
		chroma.Comment:       colors.SyntaxComment,
		chroma.Operator:      colors.SyntaxOperator,
		chroma.Punctuation:   colors.SyntaxPunct,
	})

	var out bytes.Buffer
	if err := formatter.Format(&out, style, iterator); err != nil {
		return content
	}

	return stripBackgroundSGR(out.String())
}

func stripBackgroundSGR(input string) string {
	b := strings.Builder{}
	for i := 0; i < len(input); i++ {
		if input[i] != '\x1b' || i+1 >= len(input) || input[i+1] != '[' {
			b.WriteByte(input[i])
			continue
		}

		j := i + 2
		for j < len(input) && input[j] != 'm' {
			j++
		}
		if j >= len(input) {
			b.WriteByte(input[i])
			continue
		}

		seq := input[i+2 : j]
		params := strings.Split(seq, ";")
		kept := make([]string, 0, len(params))
		skipNext := false
		for _, p := range params {
			if skipNext {
				skipNext = false
				continue
			}
			if p == "" {
				kept = append(kept, p)
				continue
			}
			n, err := strconv.Atoi(p)
			if err != nil {
				kept = append(kept, p)
				continue
			}
			if (n >= 40 && n <= 49) || (n >= 100 && n <= 109) {
				continue
			}
			if n == 48 || n == 49 || n == 58 {
				skipNext = true
				continue
			}
			kept = append(kept, p)
		}

		if len(kept) > 0 {
			b.WriteString("\x1b[")
			b.WriteString(strings.Join(kept, ";"))
			b.WriteByte('m')
		}
		i = j
	}

	return b.String()
}
