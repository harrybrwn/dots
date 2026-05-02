package main

import "strings"

func classToConstName(class string) string {
	parts := splitClassName(class)
	for i := range parts {
		parts[i] = title(parts[i])
	}
	return strings.Join(parts, "")
}

func splitClassName(class string) []string {
	split := strings.Split(class, "-")
	res := make([]string, 0, len(split))
	for i := range split {
		ix := strings.IndexByte(split[i], '_')
		if ix > 0 {
			camelParts := strings.Split(split[i], "_")
			res = append(res, camelParts...)
		} else {
			res = append(res, split[i])
		}
	}
	return res
}

func title(s string) string {
	switch s {
	case "id":
		return "ID"
	case "css":
		return "CSS"
	case "css3":
		return "CSS3"
	case "json":
		return "JSON"
	case "html":
		return "HTML"
	case "html5":
		return "HTML5"
	case "ui":
		return "UI"
	case "xml":
		return "XML"
	case "api":
		return "API"
	case "vm":
		return "VM"
	case "cpu":
		return "CPU"
	case "dns":
		return "DNS"
	case "ssh":
		return "SSH"
	case "xmpp":
		return "XMPP"
	case "ip":
		return "IP"
	case "db":
		return "DB"
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		b := []byte(s)
		c := b[0]
		c -= 'a' - 'A'
		b[0] = c
		return string(b)
	}
	return s
}
