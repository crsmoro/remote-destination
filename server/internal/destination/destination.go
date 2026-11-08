package destination

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const Warning = "Não foi possível converter o link Google Maps; a pesquisa no Waze pode indicar um destino incorreto."

var coords = regexp.MustCompile(`!3d(-?[0-9]+(?:\.[0-9]+)?)!4d(-?[0-9]+(?:\.[0-9]+)?)`)

func Normalize(input string) (string, string, error) {
	input = strings.TrimSpace(input)
	if input == "" || !utf8.ValidString(input) || utf8.RuneCountInString(input) > 4096 {
		return "", "", errors.New("entrada inválida")
	}
	values := url.Values{}
	warning := ""
	if strings.Contains(input, "://") || strings.HasPrefix(input, "//") {
		u, e := url.Parse(input)
		if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" {
			return "", "", errors.New("link inválido")
		}
		host := strings.ToLower(u.Hostname())
		switch host {
		case "waze.com", "www.waze.com", "ul.waze.com":
			if u.Path != "/ul" {
				return "", "", errors.New("link Waze inválido")
			}
			values = u.Query()
			values.Del("overview")
			values.Del("navigate")
		case "maps.app.goo.gl", "www.google.com", "maps.google.com", "goo.gl":
			valid := allowedGoogle(u)
			if !valid {
				return "", "", errors.New("link Google não permitido")
			}
			if host == "maps.app.goo.gl" || host == "goo.gl" {
				if expanded, err := expandGoogle(input); err == nil {
					if parsed, err := url.Parse(expanded); err == nil && allowedGoogle(parsed) {
						u = parsed
					}
				}
			}
			matches := coords.FindAllStringSubmatch(u.EscapedPath(), -1)
			var m []string
			if len(matches) == 1 && strings.HasPrefix(u.Path, "/maps/place/") {
				m = matches[0]
			}
			if len(m) == 3 {
				lat, e1 := strconv.ParseFloat(m[1], 64)
				lon, e2 := strconv.ParseFloat(m[2], 64)
				if e1 == nil && e2 == nil && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180 {
					values.Set("ll", m[1]+","+m[2])
				}
			}
			if values.Get("ll") == "" {
				q := u.Query()
				if q.Get("api") == "1" {
					if q.Get("destination") != "" {
						values.Set("q", q.Get("destination"))
					} else if q.Get("query") != "" {
						values.Set("q", q.Get("query"))
					}
				}
			}
			if values.Get("ll") == "" && values.Get("q") == "" {
				values.Set("q", input)
				warning = Warning
			}
		default:
			return "", "", errors.New("host não permitido")
		}
	} else {
		values.Set("q", input)
	}
	values.Set("navigate", "yes")
	return "https://waze.com/ul?" + values.Encode(), warning, nil
}
