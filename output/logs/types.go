//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package logs

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

type valueGenerator func(rng *rand.Rand, t time.Time) string

var types = map[string]valueGenerator{
	"Text":    makeText,
	"TimeT":   makeTimeT, // dangerous because time can have multiple formats.
	"IPv4":    makeIPv4,
	"IPv6":    makeIPv6,
	"UInt64":  makeInt,
	"UInt32":  makeInt,
	"UInt16":  makeInt,
	"Int64":   makeInt,
	"Int32":   makeInt,
	"UInt8":   makeByte,
	"Float64": makeFloat,
	"Float32": makeFloat,
	"MAC":     makeMAC,
}

const (
	minTextLen    = 3
	maxTextLen    = 8
	maxTimeWindow = time.Hour * 24 * 365
	maxInt        = 7890
	// Floats must accommodate durations (positive) and coordinates (<180)
	maxFloatNum = 180000
	maxFloatDiv = 1000

	text = "LoremipsumdolorsitametconsecteturadipiscingelitseddoeiusmodtemporincididuntutlaboreetdoloremagnaaliquaUtenimadminimveniamquisnostrudexercitationulamcolaborisnisiutaliquipexeacommodoconsequatDuisauteiruredolorinreprehenderitinvoluptatevelitessecillumdoloreeufugiatnulapariaturExcepteursintoccaecatcupidatatnonproidentsuntinculpaquiofficiadeseruntmollitanimidestlaborumSectionofdeFinibusBonorumetMalorumwrittenbyCiceroinBCSedutperspiciatisundeomnisistenatuserrorsitvoluptatemaccusantiumdoloremquelaudantiumtotamremaperiameaqueipsaquaeabilloinventoreveritatisetquasiarchitectobeataevitaedictasuntexplicaboNemoenimipsamvoluptatemquiavoluptassitaspernaturautoditautfugitsedquiaconsequunturmagnidoloreseosquirationevoluptatemsequinesciuntNequeporroquisquamestquidoloremipsumquiadolorsitametconsecteturadipiscivelitsedquianonnumquameiusmoditemporainciduntutlaboreetdoloremagnamaliquamquaeratvoluptatemUtenimadminimaveniamquisnostrumexercitationemullamcorporissuscipitlaboriosamnisiutaliquidexeacommodiconsequaturQuisautemveleumiurereprehenderitquiineavoluptatevelitessequamnihilmolestiaeconsequaturvelillumquidoloremeumfugiatquovoluptasnulapariatur"
)

func makeText(rng *rand.Rand, t time.Time) string {
	n := minTextLen + rng.Intn(maxTextLen-minTextLen+1)
	base := rng.Intn(len(text) - n)
	return text[base : base+n]
}

func makeTimeT(rng *rand.Rand, t time.Time) string {
	//delta := time.Duration(rng.Uint64() % uint64(maxTimeWindow))
	//return time.Now().Add(-delta).UTC().String()
	return t.UTC().Format(time.RFC3339)
}

func makeIPv4(rng *rand.Rand, t time.Time) string {
	return fmt.Sprintf("10.%d.%d.%d", rng.Intn(256), rng.Intn(256), 1+rng.Intn(253))
}

func makeInt(rng *rand.Rand, t time.Time) string {
	return strconv.Itoa(rng.Intn(maxInt))
}

func makeFloat(rng *rand.Rand, t time.Time) string {
	return fmt.Sprintf("%f", float64(rng.Intn(maxFloatNum))/float64(maxFloatDiv))
}

func makeMAC(rng *rand.Rand, t time.Time) string {
	return fmt.Sprintf("01:00:5e:%02x:%02x:%02x", rng.Intn(256), rng.Intn(256), rng.Intn(256))
}

func makeIPv6(rng *rand.Rand, t time.Time) string {
	return fmt.Sprintf("2001:db8::%04x:%04x",
		rng.Uint32(), rng.Uint32())
}

func makeByte(rng *rand.Rand, t time.Time) string {
	return strconv.Itoa(rng.Intn(256))
}

var subdomain = oneOf(
	"",
	"www.",
	"mail.",
	"internal.",
	"api.",
	"www5.",
)

var tld = oneOf(
	"com",
	"net",
	"org",
)

var ext = oneOf(
	"html",
	"htm",
	"gif",
	"jpg",
	"txt")

func makeURL(rng *rand.Rand, t time.Time) string {
	return fmt.Sprintf("https://%sexample.%s/%s/%s.%s?%s=%s#%s",
		subdomain(rng, t),
		tld(rng, t),
		makeText(rng, t),
		makeText(rng, t),
		ext(rng, t),
		makeText(rng, t),
		makeText(rng, t),
		makeText(rng, t))
}

var makeUserAgent = oneOf(
	"Mozilla/5.0 (Linux; Android 6.0; Lenovo A2016a40 Build/MRA58K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/48.0.2564.106 Mobile Safari/537.36 YaApp_Android/10.30 YaSearchBrowser/10.30",
	"Opera/9.80 (Series 60; Opera Mini/7.1.32444/174.101; U; ru) Presto/2.12.423 Version/12.16",
	"Mozilla/5.0 (Linux; Android 5.1.1; Android Build/LMY47V) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.138 Mobile Safari/537.36 YaApp_Android/9.80 YaSearchBrowser/9.80",
	"Mozilla/5.0 (Linux; Android 10; STK-L21 Build/HUAWEISTK-L21) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.83 Mobile Safari/537.36 YaApp_Android/10.91 YaSearchBrowser/10.91",
	"Mozilla/5.0 (Linux; Android 10; SM-A305FN Build/QP1A.190711.020; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/78.0.3904.96 Mobile Safari/537.36 YandexSearch/8.10 YandexSearchBrowser/8.10",
	"Mozilla/5.0 (Linux; Android 6.0; U20 Build/MRA58K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/44.0.2403.147 Mobile Safari/537.36 YaApp_Android/10.90 YaSearchBrowser/10.90",
	"Mozilla/5.0 (Linux; Android 6.0; ZTE BLADE V7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.83 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 7.0; MEIZU M6 Build/NRD90M) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3865.120 Mobile Safari/537.36 YaApp_Android/10.30 YaSearchBrowser/10.30",
	"Mozilla/5.0 (Linux; Android 9; 5024D_RU Build/PPR1.180610.011) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3865.92 Mobile Safari/537.36 YaApp_Android/10.61 YaSearchBrowser/10.61",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/80.0.3987.122 YaBrowser/20.3.0.2221 Yowser/2.5 Safari/537.36",
	"Mozilla/5.0 (Linux; Android 9; ZTE Blade V1000RU Build/PPR1.180610.011) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.138 Mobile Safari/537.36 YaApp_Android/10.91 YaSearchBrowser/10.91",
	"mobmail android 2.1.3.3150",
	"Mozilla/5.0 (compatible; Yahoo Ad monitoring; https://help.yahoo.com/kb/yahoo-ad-monitoring-SLN24857.html) yahoo.adquality.lwd.desktop/1591143192-10",
	"Mozilla/5.0 (Linux; Android 9; Pixel 3 Build/PD1A.180720.030) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/66.0.3359.158 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 9; POCOPHONE F1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.83 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 9; Notepad_K10) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.83 Safari/537.36",
	"Mozilla/5.0 (Linux; Android 8.0.0; VS996) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.83 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; U; Android 7.1.2; uz-uz; Redmi 4X Build/N2G47H) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/71.0.3578.141 Mobile Safari/537.36 XiaoMi/MiuiBrowser/12.2.3-g",
	"Mozilla/5.0 (Linux; Android 10; LM-V350) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.83 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 9; U307AS) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.83 Mobile Safari/537.36",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 13_4_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 LightSpeed [FBAN/MessengerLiteForiOS;FBAV/266.0.0.32.114;FBBV/216059178;FBDV/iPhone10,6;FBMD/iPhone;FBSN/iOS;FBSV/13.4.1;FBSS/3;FBCR/;FBID/phone;FBLC/en_US;FBOP/0]",
	"Mozilla/5.0 (Linux; Android 10; ASUS_X01BDA) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/80.0.3987.162 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 6.0; QMobile X700 PRO II) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3865.92 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 4.1.2; Micromax P410i Build/JZO54K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.111 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 10; SM-A715F Build/QP1A.190711.020; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/83.0.4103.83 Mobile Safari/537.36 [FB_IAB/Orca-Android;FBAV/266.0.0.16.117;]",
	"Mozilla/5.0 (Linux; Android 9; LG-US998) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.83 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; U; Android 4.0.3; es-us; GT-P3100 Build/IML74K) AppleWebKit/534.30 (KHTML, like Gecko) Version/4.0 Safari/534.30",
	"Mozilla/5.0 (Linux; Android 9; G8142) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.83 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 8.1.0; SM-A260G Build/OPR6; rv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Rocket/2.1.17(19420) Chrome/81.0.4044.138 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 7.0; SM-S337TL) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.83 Mobile Safari/537.36")

var makeHTTPMethod = oneOf(
	"GET",
	"POST",
	"PUT",
	"OPTIONS",
	"HEAD",
)

var hostName = oneOf(
	"localhost",
	"example",
	"test",
	"invalid",
	"local",
	"localdomain",
	"domain",
	"lan",
	"home",
	"host",
	"corp",
)

func makeHostName(rng *rand.Rand, t time.Time) string {
	return fmt.Sprintf("%s%s.%s%s",
		makeText(rng, t),
		makeInt(rng, t),
		subdomain(rng, t),
		hostName(rng, t))
}

func join(generators ...valueGenerator) valueGenerator {
	return func(rng *rand.Rand, t time.Time) string {
		var sb strings.Builder
		for _, gen := range generators {
			sb.WriteString(gen(rng, t))
		}
		return sb.String()
	}
}

func ct(s string) valueGenerator {
	return func(rng *rand.Rand, t time.Time) string {
		return s
	}
}

var makeInterface = join(
	oneOf("eth", "enp0s", "lo"),
	makeInt,
)

var makeTimezone = oneOf(
	"CET",
	"CEST",
	"OMST",
	"ET",
	"CT",
	"PT",
	"PST",
	"GMT+02:00",
	"GMT-07:00",
)
