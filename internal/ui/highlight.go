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
		chroma.Background:               "bg:" + colors.Bg,
		chroma.Text:                     colors.SyntaxPlain,
		chroma.Other:                    colors.SyntaxPlain,
		chroma.Keyword:                  colors.SyntaxKeyword,
		chroma.KeywordConstant:          colors.SyntaxKeyword,
		chroma.KeywordDeclaration:       colors.SyntaxKeyword,
		chroma.KeywordNamespace:         colors.SyntaxKeyword,
		chroma.KeywordPseudo:            colors.SyntaxKeyword,
		chroma.KeywordReserved:          colors.SyntaxKeyword,
		chroma.KeywordType:              colors.SyntaxType,
		chroma.Name:                     colors.SyntaxPlain,
		chroma.NameAttribute:            colors.SyntaxPlain,
		chroma.NameBuiltin:              colors.SyntaxFunc,
		chroma.NameBuiltinPseudo:        colors.SyntaxFunc,
		chroma.NameClass:                colors.SyntaxType,
		chroma.NameConstant:             colors.SyntaxNumber,
		chroma.NameDecorator:            colors.SyntaxFunc,
		chroma.NameException:            colors.SyntaxType,
		chroma.NameFunction:             colors.SyntaxFunc,
		chroma.NameFunctionMagic:        colors.SyntaxFunc,
		chroma.NameNamespace:            colors.SyntaxType,
		chroma.NameProperty:             colors.SyntaxPlain,
		chroma.NameTag:                  colors.SyntaxType,
		chroma.NameVariable:             colors.SyntaxPlain,
		chroma.NameVariableClass:        colors.SyntaxPlain,
		chroma.NameVariableGlobal:       colors.SyntaxPlain,
		chroma.NameVariableInstance:     colors.SyntaxPlain,
		chroma.NameVariableMagic:        colors.SyntaxPlain,
		chroma.Literal:                  colors.SyntaxPlain,
		chroma.LiteralDate:              colors.SyntaxString,
		chroma.LiteralString:            colors.SyntaxString,
		chroma.LiteralStringAffix:       colors.SyntaxString,
		chroma.LiteralStringBacktick:    colors.SyntaxString,
		chroma.LiteralStringChar:        colors.SyntaxString,
		chroma.LiteralStringDelimiter:   colors.SyntaxString,
		chroma.LiteralStringDoc:         colors.SyntaxComment,
		chroma.LiteralStringDouble:      colors.SyntaxString,
		chroma.LiteralStringEscape:      colors.SyntaxString,
		chroma.LiteralStringHeredoc:     colors.SyntaxString,
		chroma.LiteralStringInterpol:    colors.SyntaxString,
		chroma.LiteralStringOther:       colors.SyntaxString,
		chroma.LiteralStringRegex:       colors.SyntaxString,
		chroma.LiteralStringSingle:      colors.SyntaxString,
		chroma.LiteralStringSymbol:      colors.SyntaxString,
		chroma.LiteralNumber:            colors.SyntaxNumber,
		chroma.LiteralNumberBin:         colors.SyntaxNumber,
		chroma.LiteralNumberFloat:       colors.SyntaxNumber,
		chroma.LiteralNumberHex:         colors.SyntaxNumber,
		chroma.LiteralNumberInteger:     colors.SyntaxNumber,
		chroma.LiteralNumberIntegerLong: colors.SyntaxNumber,
		chroma.LiteralNumberOct:         colors.SyntaxNumber,
		chroma.Operator:                 colors.SyntaxOperator,
		chroma.OperatorWord:             colors.SyntaxOperator,
		chroma.Punctuation:              colors.SyntaxPunct,
		chroma.Comment:                  colors.SyntaxComment,
		chroma.CommentHashbang:          colors.SyntaxComment,
		chroma.CommentMultiline:         colors.SyntaxComment,
		chroma.CommentPreproc:           colors.SyntaxComment,
		chroma.CommentPreprocFile:       colors.SyntaxComment,
		chroma.CommentSingle:            colors.SyntaxComment,
		chroma.CommentSpecial:           colors.SyntaxComment,
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
		for idx := 0; idx < len(params); idx++ {
			p := params[idx]
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
			if n == 48 || n == 58 {
				idx = skipExtendedColorParams(params, idx)
				continue
			}
			if n == 49 || n == 59 {
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

func skipExtendedColorParams(params []string, idx int) int {
	if idx+1 >= len(params) {
		return idx
	}
	mode, err := strconv.Atoi(params[idx+1])
	if err != nil {
		return idx
	}
	switch mode {
	case 5:
		if idx+2 < len(params) {
			return idx + 2
		}
	case 2:
		if idx+4 < len(params) {
			return idx + 4
		}
	}
	return idx + 1
}
