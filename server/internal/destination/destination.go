package destination

import (
	"errors"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const Warning = "Não foi possível converter o link Google Maps; a pesquisa no Waze pode indicar um destino incorreto."

var coords = regexp.MustCompile(`!3d(-?[0-9]+(?:\.[0-9]+)?)!4d(-?[0-9]+(?:\.[0-9]+)?)`)
var mobileFtid = regexp.MustCompile(`^0x[0-9a-fA-F]{1,16}:0x[0-9a-fA-F]{1,16}$`)
var mobileNumeric = regexp.MustCompile(`^[+\-0-9.,\s]+$`)
var percentEscape = regexp.MustCompile(`%[0-9a-fA-F]{2}`)

// mobileMapsQuery handles the explicit mobile place query, not a viewport or ftid.
func mobileMapsQuery(u *url.URL) (string, string, error) {
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(q["q"]) != 1 || len(q["ftid"]) != 1 || !mobileFtid.MatchString(q.Get("ftid")) {
		return "", "", errors.New("invalid mobile Maps query")
	}
	for k, v := range q {
		switch k {
		case "q", "ftid", "entry", "shh", "lucs", "g_ep", "skid", "g_st":
		default:
			return "", "", errors.New("unsupported mobile Maps parameter")
		}
		if len(v) != 1 || !utf8.ValidString(v[0]) || strings.IndexFunc(v[0], unicode.IsControl) >= 0 {
			return "", "", errors.New("invalid mobile Maps parameter")
		}
	}
	text := q.Get("q")
	if strings.TrimSpace(text) == "" {
		return "", "", errors.New("invalid mobile Maps destination")
	}
	// Inspect additional percent-encoded layers for smuggled URLs without
	// changing the once-decoded destination passed to the geocoder.
	for inspected := text; ; {
		lower := strings.ToLower(strings.TrimSpace(inspected))
		if strings.Contains(lower, "://") || strings.Contains(lower, "http:") || strings.Contains(lower, "https:") || strings.HasPrefix(lower, "//") {
			return "", "", errors.New("invalid mobile Maps destination")
		}
		next := percentEscape.ReplaceAllStringFunc(inspected, func(escape string) string {
			decoded, _ := url.PathUnescape(escape)
			return decoded
		})
		if next == inspected {
			break
		}
		inspected = next
	}
	if strings.Contains(text, ",") && mobileNumeric.MatchString(text) {
		parts := strings.Split(text, ",")
		if len(parts) != 2 {
			return "", "", errors.New("invalid coordinates")
		}
		lat, e1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		lon, e2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if e1 != nil || e2 != nil || math.IsNaN(lat) || math.IsNaN(lon) || math.Abs(lat) > 90 || math.Abs(lon) > 180 {
			return "", "", errors.New("invalid coordinates")
		}
		return "ll", strconv.FormatFloat(lat, 'f', -1, 64) + "," + strconv.FormatFloat(lon, 'f', -1, 64), nil
	}
	return "q", text, nil
}

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
			if u.Path == "/maps" && (u.Query().Has("q") || u.Query().Has("ftid")) {
				key, value, err := mobileMapsQuery(u)
				if err != nil {
					return "", "", err
				}
				values.Set(key, value)
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
						query := q.Get("query")
						if (u.Path == "/maps/search/" || u.Path == "/maps/search") && len(q["api"]) == 1 && len(q["query"]) == 1 {
							parts := strings.Split(query, ",")
							if len(parts) == 2 {
								lat, e1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
								lon, e2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
								if e1 == nil && e2 == nil && !math.IsNaN(lat) && !math.IsNaN(lon) && math.Abs(lat) <= 90 && math.Abs(lon) <= 180 {
									values.Set("ll", strconv.FormatFloat(lat, 'f', -1, 64)+","+strconv.FormatFloat(lon, 'f', -1, 64))
								}
							}
						}
						if values.Get("ll") == "" {
							values.Set("q", query)
						}
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

// SafeCoordinateLink accepts only proven coordinate destinations, never text.
func SafeCoordinateLink(input, normalized string) bool {
	u, err := url.Parse(strings.TrimSpace(input))
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "waze.com" || host == "www.waze.com" || host == "ul.waze.com" {
		q, err := url.ParseQuery(u.RawQuery)
		if err != nil || u.Path != "/ul" {
			return false
		}
		for k, v := range q {
			if len(v) != 1 || (k != "ll" && k != "navigate" && k != "overview") {
				return false
			}
			if k == "navigate" && v[0] != "yes" && v[0] != "no" {
				return false
			}
			if k == "overview" && v[0] != "yes" && v[0] != "no" {
				return false
			}
		}
	} else if host != "www.google.com" && host != "maps.google.com" && host != "maps.app.goo.gl" && host != "goo.gl" {
		return false
	}
	n, err := url.Parse(normalized)
	if err != nil {
		return false
	}
	q, err := url.ParseQuery(n.RawQuery)
	if err != nil || len(q["ll"]) != 1 || q.Get("q") != "" {
		return false
	}
	parts := strings.Split(q.Get("ll"), ",")
	if len(parts) != 2 {
		return false
	}
	lat, e1 := strconv.ParseFloat(parts[0], 64)
	lon, e2 := strconv.ParseFloat(parts[1], 64)
	return e1 == nil && e2 == nil && !math.IsNaN(lat) && !math.IsNaN(lon) && math.Abs(lat) <= 90 && math.Abs(lon) <= 180
}
