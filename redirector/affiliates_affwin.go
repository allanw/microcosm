package redirector

import (
	"net/url"
	"strconv"

	"github.com/golang/glog"

	"github.com/microcosm-cc/microcosm/models"
)

// This must never be changed, this is how we make money
const affWinAffiliateID string = "101164"

var affwinDomainParts = []string{
	".awin1.",
	".chainreactioncycles.",
	".cyclestore.",
	".evanscycles.",
	".hargrovescycles.",
	".howies.",
	".merlincycles.",
	".probikekit.",
	".ribblecycles.",
	".rutlandcycling.",
	".wiggle",
}

type affWinLink struct {
	Link models.Link
}

func (m *affWinLink) getDestination() (bool, string) {

	// Hijack an existing affiliate link
	if m.Link.Domain == "www.awin1.com" {
		u, err := url.Parse(m.Link.Url)
		if err != nil {
			glog.Errorf("url.Parse(`%s`) %+v", m.Link.Url, err)
			return false, m.Link.Url
		}

		q := u.Query()
		q.Del("awinaffid")
		q.Add("awinaffid", affWinAffiliateID)
		u.RawQuery = q.Encode()

		return true, u.String()
	}

	// Fetch a program ID based on domain
	var programID int
	switch m.Link.Domain {
	case "www.chainreactioncycles.com":
		programID = 2698
	case "www.cyclestore.co.uk":
		programID = 3462
	case "www.evanscycles.com":
		programID = 1302
	case "www.hargrovescycles.co.uk":
		programID = 2828
	case "www.howies.co.uk":
		programID = 3167
	case "www.merlincycles.co.uk":
		programID = 3361
	case "www.probikekit.co.uk":
		programID = 3977
	case "www.probikekit.com":
		programID = 3977
	case "www.ribblecycles.co.uk":
		programID = 5923
	case "www.rutlandcycling.com":
		programID = 3395
	case "www.wiggle.co.uk":
		programID = 1857
	case "www.wiggle.es":
		programID = 1857
	case "www.wiggle.cn":
		programID = 1857
	case "www.wiggle.com":
		programID = 1857
	case "www.wiggle.com.au":
		programID = 1857
	case "www.wiggle.fr":
		programID = 1857
	case "www.wigglesport.it":
		programID = 1857
	case "www.wigglesport.de":
		programID = 1857
	case "www.wiggle.jp":
		programID = 1857
	case "www.wiggle.ru":
		programID = 1857
	case "www.wiggle.pt":
		programID = 1857

	default:
		return false, m.Link.Url
	}

	if programID == 3977 {
		u, _ := url.Parse(m.Link.Url)
		q := u.Query()
		q.Del("affil")
		u.RawQuery = q.Encode()
		m.Link.Url = u.String()
	}

	u, _ := url.Parse("http://www.awin1.com/cread.php")
	q := u.Query()
	q.Add("awinaffid", affWinAffiliateID)
	q.Add("awinmid", strconv.Itoa(programID))
	q.Add("clickref", "")
	q.Add("p", m.Link.Url)
	u.RawQuery = q.Encode()

	return true, u.String()
}
