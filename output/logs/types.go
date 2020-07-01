//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package logs

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

type valueGenerator func(rng *rand.Rand, t time.Time) string

var types = map[string]valueGenerator{
	"Text":    makeText,
	"TimeT":   makeTimeT,
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
	return t.String()
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
