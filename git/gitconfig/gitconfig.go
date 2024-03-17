package gitconfig

import (
	"errors"
)

const utf8BOM = "\357\273\277"

var (
	// ErrInvalidEscapeSequence indicates that the escape character ('\')
	// was followed by an invalid character.
	ErrInvalidEscapeSequence = errors.New("unknown escape sequence")
	// ErrUnfinishedQuote indicates that a value has an odd number of (unescaped) quotes
	ErrUnfinishedQuote = errors.New("unfinished quote")
	// ErrMissingEquals indicates that an equals sign ('=') was expected but not found
	ErrMissingEquals = errors.New("expected '='")
	// ErrPartialBOM indicates that the file begins with a partial UTF8-BOM
	ErrPartialBOM = errors.New("partial UTF8-BOM")
	// ErrInvalidKeyChar indicates that there was an invalid key character
	ErrInvalidKeyChar = errors.New("invalid key character")
	// ErrInvalidSectionChar indicates that there was an invalid character in section
	ErrInvalidSectionChar = errors.New("invalid character in section")
	// ErrUnexpectedEOF indicates that there was an unexpected EOF
	ErrUnexpectedEOF = errors.New("unexpected EOF")
	// ErrSectionNewLine indicates that there was a newline in section
	ErrSectionNewLine = errors.New("newline in section")
	// ErrMissingStartQuote indicates that there was a missing start quote
	ErrMissingStartQuote = errors.New("missing start quote")
	// ErrMissingClosingBracket indicates that there was a missing closing bracket in section
	ErrMissingClosingBracket = errors.New("missing closing section bracket")
)

type configParser struct {
	bytes  []byte
	linenr uint
	eof    bool
}

// Parse takes given bytes as configuration file (according to gitconfig syntax)
func Parse(bytes []byte) (*Config, uint, error) {
	parser := &configParser{bytes, 1, false}
	cfg, err := parser.parse()
	return cfg, parser.linenr, err
}

type Config struct {
	sections map[string]*Section
}

func (c *Config) MarshalText() ([]byte, error) {
	return nil, nil
}

func (c *Config) GetSection(name string) *Section {
	return c.sections[name]
}

type Section struct {
	name        string
	entries     map[string]string
	subsections map[string]*Section
}

func (cf *configParser) parse() (*Config, error) {
	bomPtr := 0
	comment := false
	name := ""
	var (
		section *Section
		cnf     = &Config{sections: make(map[string]*Section)}
	)
	for {
		c := cf.nextChar()
		if bomPtr != -1 && bomPtr < len(utf8BOM) {
			if c == (utf8BOM[bomPtr] & 0377) {
				bomPtr++
				continue
			} else {
				/* Do not tolerate partial BOM. */
				if bomPtr != 0 {
					return cnf, ErrPartialBOM
				}
				bomPtr = -1
			}
		}
		if c == '\n' {
			if cf.eof {
				return cnf, nil
			}
			comment = false
			continue
		}
		if comment || isspace(c) {
			continue
		}
		if c == '#' || c == ';' {
			comment = true
			continue
		}
		if c == '[' {
			sect, ext, err := cf.getSectionFullName()
			if err != nil {
				return cnf, err
			}
			var ok bool
			if section, ok = cnf.sections[sect]; !ok {
				section = &Section{
					name:        sect,
					entries:     make(map[string]string),
					subsections: make(map[string]*Section),
				}
			}
			cnf.sections[sect] = section
			if len(ext) > 0 {
				sub := Section{name: ext, entries: make(map[string]string)}
				section.subsections[ext] = &sub
				section = &sub
			}
			continue
		}
		if !isalpha(c) {
			return cnf, ErrInvalidKeyChar
		}
		key := name + string(c)
		value, err := cf.getValue(&key)
		if err != nil {
			return cnf, err
		}
		section.entries[key] = value
	}
}

func (cf *configParser) dumbParse() (map[string]string, error) {
	bomPtr := 0
	comment := false
	cfg := map[string]string{}
	name := ""
	for {
		c := cf.nextChar()
		if bomPtr != -1 && bomPtr < len(utf8BOM) {
			if c == (utf8BOM[bomPtr] & 0377) {
				bomPtr++
				continue
			} else {
				/* Do not tolerate partial BOM. */
				if bomPtr != 0 {
					return cfg, ErrPartialBOM
				}
				bomPtr = -1
			}
		}
		if c == '\n' {
			if cf.eof {
				return cfg, nil
			}
			comment = false
			continue
		}
		if comment || isspace(c) {
			continue
		}
		if c == '#' || c == ';' {
			comment = true
			continue
		}
		if c == '[' {
			// name, err = cf.getSectionKey()
			// if err != nil {
			// 	return cfg, err
			// }
			// name += "."
			sect, ext, err := cf.getSectionFullName()
			if err != nil {
				return cfg, err
			}
			if len(ext) > 0 {
				name = sect + "." + ext + "."
			} else {
				name = sect + "."
			}
			continue
		}
		if !isalpha(c) {
			return cfg, ErrInvalidKeyChar
		}
		key := name + string(c)
		value, err := cf.getValue(&key)
		if err != nil {
			return cfg, err
		}
		cfg[key] = value
	}
}

func (cf *configParser) nextChar() byte {
	if len(cf.bytes) == 0 {
		cf.eof = true
		return byte('\n')
	}
	c := cf.bytes[0]
	if c == '\r' {
		/* DOS like systems */
		if len(cf.bytes) > 1 && cf.bytes[1] == '\n' {
			cf.bytes = cf.bytes[1:]
			c = '\n'
		}
	}
	if c == '\n' {
		cf.linenr++
	}
	if len(cf.bytes) == 0 {
		cf.eof = true
		cf.linenr++
		c = '\n'
	}
	cf.bytes = cf.bytes[1:]
	return c
}

func (cf *configParser) getSectionFullName() (name string, ext string, err error) {
	for {
		c := cf.nextChar()
		if cf.eof {
			return "", "", ErrUnexpectedEOF
		}
		if c == ']' {
			return
		}
		if isspace(c) {
			ext, err = cf.getExtendedSectionKey(c)
			return name, ext, err
		}
		if !iskeychar(c) && c != '.' {
			return "", "", ErrInvalidSectionChar
		}
		name += string(lower(c))
	}
}

func (cf *configParser) getSectionKey() (string, error) {
	name := ""
	for {
		c := cf.nextChar()
		if cf.eof {
			return "", ErrUnexpectedEOF
		}
		if c == ']' {
			return name, nil
		}
		if isspace(c) {
			ext, err := cf.getExtendedSectionKey(c)
			if err != nil {
				return "", err
			}
			return name + "." + ext, nil
		}
		if !iskeychar(c) && c != '.' {
			return "", ErrInvalidSectionChar
		}
		name += string(lower(c))
	}
}

// config: [BaseSection "ExtendedSection"]
func (cf *configParser) getExtendedSectionKey(c byte) (ext string, err error) {
	for {
		if c == '\n' {
			cf.linenr--
			return "", ErrSectionNewLine
		}
		c = cf.nextChar()
		if !isspace(c) {
			break
		}
	}
	if c != '"' {
		return "", ErrMissingStartQuote
	}
	for {
		c = cf.nextChar()
		if c == '\n' {
			cf.linenr--
			return "", ErrSectionNewLine
		}
		if c == '"' {
			break
		}
		if c == '\\' {
			c = cf.nextChar()
			if c == '\n' {
				cf.linenr--
				return "", ErrSectionNewLine
			}
		}
		ext += string(c)
	}
	if cf.nextChar() != ']' {
		return "", ErrMissingClosingBracket
	}
	return ext, nil
}

func (cf *configParser) getValue(name *string) (string, error) {
	var c byte
	var err error
	var value string

	/* Get the full name */
	for {
		c = cf.nextChar()
		if cf.eof {
			break
		}
		if !iskeychar(c) {
			break
		}
		*name += string(lower(c))
	}

	for c == ' ' || c == '\t' {
		c = cf.nextChar()
	}

	if c != '\n' {
		if c != '=' {
			return "", ErrInvalidKeyChar
		}
		value, err = cf.parseValue()
		if err != nil {
			return "", err
		}
	}
	/*
	 * We already consumed the \n, but we need linenr to point to
	 * the line we just parsed during the call to fn to get
	 * accurate line number in error messages.
	 */
	// cf.linenr--
	// ret := fn(name->buf, value, data);
	// if ret >= 0 {
	// 	cf.linenr++
	// }
	return value, err
}

func (cf *configParser) parseValue() (string, error) {
	var quote, comment bool
	var space int

	var value string

	// strbuf_reset(&cf->value);
	for {
		c := cf.nextChar()
		if c == '\n' {
			if quote {
				cf.linenr--
				return "", ErrUnfinishedQuote
			}
			return value, nil
		}
		if comment {
			continue
		}
		if isspace(c) && !quote {
			if len(value) > 0 {
				space++
			}
			continue
		}
		if !quote {
			if c == ';' || c == '#' {
				comment = true
				continue
			}
		}
		for space != 0 {
			value += " "
			space--
		}
		if c == '\\' {
			c = cf.nextChar()
			switch c {
			case '\n':
				continue
			case 't':
				c = '\t'
				break
			case 'b':
				c = '\b'
				break
			case 'n':
				c = '\n'
				break
			/* Some characters escape as themselves */
			case '\\':
				break
			case '"':
				break
			/* Reject unknown escape sequences */
			default:
				return "", ErrInvalidEscapeSequence
			}
			value += string(c)
			continue
		}
		if c == '"' {
			quote = !quote
			continue
		}
		value += string(c)
	}
}

func lower(c byte) byte {
	return c | 0x20
}

func isspace(c byte) bool {
	return c == '\t' || c == ' ' || c == '\n' || c == '\v' || c == '\f' || c == '\r'
}

func iskeychar(c byte) bool {
	return isalnum(c) || c == '-'
}

func isalnum(c byte) bool {
	return isalpha(c) || isnum(c)
}

func isalpha(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

func isnum(c byte) bool {
	return c >= '0' && c <= '9'
}
