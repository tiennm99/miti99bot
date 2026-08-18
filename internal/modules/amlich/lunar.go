package amlich

import (
	"fmt"
	"math"
)

// This file ports Hồ Ngọc Đức's lunar-calendar algorithm
// (https://www.informatik.uni-leipzig.de/~duc/amlich/), the de-facto reference
// for the Vietnamese calendar. Lunar months start on the day of the
// astronomical new moon at UTC+7; month numbering is anchored on the month
// containing the winter solstice (lunar month 11), and a year with 13 lunations
// repeats the first month without a major solar term as a leap month (tháng
// nhuận). Computing at any other UTC offset shifts month boundaries — this is
// why the Vietnamese and Chinese calendars occasionally disagree.

// lunarTimeZone is the UTC offset (hours) the Vietnamese calendar is defined
// against.
const lunarTimeZone = 7.0

const (
	// newMoonCycle is the mean synodic month length in days.
	newMoonCycle = 29.530588853
	// jdNewMoonEpoch is the Julian day of the reference new moon (Jan 1900)
	// that lunation indices count from.
	jdNewMoonEpoch = 2415021.076998695
)

// minLunarYear/maxLunarYear bound both commands. The astronomical formulas
// stay accurate well beyond this window, but the published Vietnamese
// reference tables cover 1800–2199, so claims outside it are unverifiable.
const (
	minLunarYear = 1800
	maxLunarYear = 2199
)

// floorInt mirrors the reference implementation's INT() — floor, not
// truncation, which differs for the negative lunation indices of pre-1900
// dates.
func floorInt(x float64) int { return int(math.Floor(x)) }

// jdFromDate returns the Julian day number of dd/mm/yy at noon. Handles the
// Julian→Gregorian switch, though all supported years are Gregorian.
func jdFromDate(dd, mm, yy int) int {
	a := (14 - mm) / 12
	y := yy + 4800 - a
	m := mm + 12*a - 3
	jd := dd + (153*m+2)/5 + 365*y + y/4 - y/100 + y/400 - 32045
	if jd < 2299161 {
		jd = dd + (153*m+2)/5 + 365*y + y/4 - 32083
	}
	return jd
}

// jdToDate is the inverse of jdFromDate.
func jdToDate(jd int) (dd, mm, yy int) {
	var a, b, c int
	if jd > 2299160 {
		a = jd + 32044
		b = (4*a + 3) / 146097
		c = a - (b*146097)/4
	} else {
		c = jd + 32082
	}
	d := (4*c + 3) / 1461
	e := c - (1461*d)/4
	m := (5*e + 2) / 153
	dd = e - (153*m+2)/5 + 1
	mm = m + 3 - 12*(m/10)
	yy = b*100 + d - 4800 + m/10
	return dd, mm, yy
}

// newMoonJd returns the Julian day (with fraction, UTC) of the k-th new moon
// after the reference epoch, using the truncated series from Jean Meeus,
// "Astronomical Algorithms".
func newMoonJd(k int) float64 {
	kf := float64(k)
	T := kf / 1236.85 // time in Julian centuries from 1900 January 0.5
	T2 := T * T
	T3 := T2 * T
	dr := math.Pi / 180
	jd1 := 2415020.75933 + 29.53058868*kf + 0.0001178*T2 - 0.000000155*T3
	jd1 += 0.00033 * math.Sin((166.56+132.87*T-0.009173*T2)*dr)
	M := 359.2242 + 29.10535608*kf - 0.0000333*T2 - 0.00000347*T3    // sun's mean anomaly
	Mpr := 306.0253 + 385.81691806*kf + 0.0107306*T2 + 0.00001236*T3 // moon's mean anomaly
	F := 21.2964 + 390.67050646*kf - 0.0016528*T2 - 0.00000239*T3    // moon's argument of latitude
	c1 := (0.1734-0.000393*T)*math.Sin(M*dr) + 0.0021*math.Sin(2*M*dr)
	c1 -= 0.4068 * math.Sin(Mpr*dr)
	c1 += 0.0161 * math.Sin(2*Mpr*dr)
	c1 -= 0.0004 * math.Sin(3*Mpr*dr)
	c1 += 0.0104 * math.Sin(2*F*dr)
	c1 -= 0.0051 * math.Sin((M+Mpr)*dr)
	c1 -= 0.0074 * math.Sin((M-Mpr)*dr)
	c1 += 0.0004 * math.Sin((2*F+M)*dr)
	c1 -= 0.0004 * math.Sin((2*F-M)*dr)
	c1 -= 0.0006 * math.Sin((2*F+Mpr)*dr)
	c1 += 0.0010 * math.Sin((2*F-Mpr)*dr)
	c1 += 0.0005 * math.Sin((2*Mpr+M)*dr)
	var deltat float64
	if T < -11 {
		deltat = 0.001 + 0.000839*T + 0.0002261*T2 - 0.00000845*T3 - 0.000000081*T*T3
	} else {
		deltat = -0.000278 + 0.000265*T + 0.000262*T2
	}
	return jd1 + c1 - deltat
}

// getNewMoonDay returns the calendar day (Julian day number at UTC+7) on which
// the k-th new moon falls.
func getNewMoonDay(k int) int {
	return floorInt(newMoonJd(k) + 0.5 + lunarTimeZone/24)
}

// sunLongitude returns the sun's apparent ecliptic longitude in radians
// [0, 2π) at the instant jdn.
func sunLongitude(jdn float64) float64 {
	T := (jdn - 2451545.0) / 36525 // Julian centuries from J2000
	T2 := T * T
	dr := math.Pi / 180
	M := 357.52910 + 35999.05030*T - 0.0001559*T2 - 0.00000048*T*T2
	L0 := 280.46645 + 36000.76983*T + 0.0003032*T2
	DL := (1.914600 - 0.004817*T - 0.000014*T2) * math.Sin(dr*M)
	DL += (0.019993-0.000101*T)*math.Sin(dr*2*M) + 0.000290*math.Sin(dr*3*M)
	L := (L0 + DL) * dr
	L -= 2 * math.Pi * math.Floor(L/(2*math.Pi))
	return L
}

// getSunLongitude returns the major-term index (0..11, one per 30° of solar
// longitude) at local midnight beginning the given calendar day. A lunar month
// containing no major-term transition is the leap month.
func getSunLongitude(dayNumber int) int {
	return floorInt(sunLongitude(float64(dayNumber)-0.5-lunarTimeZone/24) / math.Pi * 6)
}

// getLunarMonth11 returns the start day of lunar month 11 of year yy — the
// month containing the winter solstice (sun longitude 270°, term index 9).
func getLunarMonth11(yy int) int {
	off := jdFromDate(31, 12, yy) - 2415021
	k := floorInt(float64(off) / newMoonCycle)
	nm := getNewMoonDay(k)
	if getSunLongitude(nm) >= 9 {
		nm = getNewMoonDay(k - 1)
	}
	return nm
}

// getLeapMonthOffset returns how many months after month 11 (whose start day
// is a11) the leap month begins — the first month with no major-term
// transition. Only meaningful in a 13-lunation year.
func getLeapMonthOffset(a11 int) int {
	k := floorInt((float64(a11)-jdNewMoonEpoch)/newMoonCycle + 0.5)
	i := 1 // start with the month following lunar month 11
	arc := getSunLongitude(getNewMoonDay(k + i))
	for {
		last := arc
		i++
		arc = getSunLongitude(getNewMoonDay(k + i))
		if arc == last || i >= 14 {
			break
		}
	}
	return i - 1
}

// solarToLunar converts a Gregorian date to its Vietnamese lunar date.
// leap reports whether the resulting month is the leap month (tháng nhuận).
func solarToLunar(dd, mm, yy int) (day, month, year int, leap bool) {
	dayNumber := jdFromDate(dd, mm, yy)
	k := floorInt((float64(dayNumber) - jdNewMoonEpoch) / newMoonCycle)
	// The mean-cycle estimate k can overshoot the true lunation by one when
	// dayNumber falls just before a new moon. The reference implementation
	// steps back only once and thus returns day 0 for a handful of days
	// (e.g. 13/4/1877, 9/4/2062); loop until the month start is on or before
	// the target day.
	monthStart := getNewMoonDay(k + 1)
	for monthStart > dayNumber {
		k--
		monthStart = getNewMoonDay(k + 1)
	}
	a11 := getLunarMonth11(yy)
	b11 := a11
	if a11 >= monthStart {
		year = yy
		a11 = getLunarMonth11(yy - 1)
	} else {
		year = yy + 1
		b11 = getLunarMonth11(yy + 1)
	}
	day = dayNumber - monthStart + 1
	diff := (monthStart - a11) / 29
	month = diff + 11
	if b11-a11 > 365 { // 13 lunations between successive month-11 starts → leap year
		leapMonthDiff := getLeapMonthOffset(a11)
		if diff >= leapMonthDiff {
			month = diff + 10
			if diff == leapMonthDiff {
				leap = true
			}
		}
	}
	if month > 12 {
		month -= 12
	}
	if month >= 11 && diff < 4 {
		year--
	}
	return day, month, year, leap
}

// lunarToSolar converts a Vietnamese lunar date to Gregorian. Unlike the
// reference implementation (which silently returns a wrong date), it rejects
// impossible inputs — a leap month the year does not have, or day 30 of a
// 29-day month — with a user-ready Vietnamese message.
func lunarToSolar(day, month, year int, leap bool) (dd, mm, yy int, err error) {
	var a11, b11 int
	if month < 11 {
		a11 = getLunarMonth11(year - 1)
		b11 = getLunarMonth11(year)
	} else {
		a11 = getLunarMonth11(year)
		b11 = getLunarMonth11(year + 1)
	}
	k := floorInt(0.5 + (float64(a11)-jdNewMoonEpoch)/newMoonCycle)
	off := month - 11
	if off < 0 {
		off += 12
	}
	if b11-a11 > 365 {
		leapOff := getLeapMonthOffset(a11)
		leapMonth := leapOff - 2
		if leapMonth < 0 {
			leapMonth += 12
		}
		if leap && month != leapMonth {
			return 0, 0, 0, fmt.Errorf("năm âm lịch %d không có tháng %d nhuận", year, month)
		}
		if leap || off >= leapOff {
			off++
		}
	} else if leap {
		return 0, 0, 0, fmt.Errorf("năm âm lịch %d không có tháng nhuận", year)
	}
	monthStart := getNewMoonDay(k + off)
	monthDays := getNewMoonDay(k+off+1) - monthStart
	if day > monthDays {
		return 0, 0, 0, fmt.Errorf("tháng %d%s năm âm lịch %d chỉ có %d ngày", month, leapLabel(leap), year, monthDays)
	}
	dd, mm, yy = jdToDate(monthStart + day - 1)
	return dd, mm, yy, nil
}

// disputedMonthStarts holds the Julian day numbers of the lunar-month starts
// whose defining new moon falls within ~2 minutes of UTC+7 midnight, where
// ΔT uncertainty exceeds the margin — the true boundary may sit one day
// earlier in future official tables (see docs/amlich-known-issues.md, which
// lists the disputed new-moon days themselves; this engine begins each of
// these months on the following day). Conversions touching these months carry
// a caveat in the bot's reply.
var disputedMonthStarts = map[int]bool{
	jdFromDate(10, 12, 2072): true,
	jdFromDate(16, 11, 2077): true,
	jdFromDate(8, 5, 2130):   true,
	jdFromDate(27, 5, 2150):  true,
	jdFromDate(18, 5, 2159):  true,
	jdFromDate(23, 1, 2175):  true,
	jdFromDate(27, 1, 2199):  true,
}

// nearDisputedBoundary reports whether the lunar month containing the solar
// day jdn starts or ends on a disputed boundary — i.e. whether the conversion
// result for that day could shift by one day against future official tables.
func nearDisputedBoundary(jdn int) bool {
	k := floorInt((float64(jdn) - jdNewMoonEpoch) / newMoonCycle)
	monthStart := getNewMoonDay(k + 1)
	for monthStart > jdn {
		k--
		monthStart = getNewMoonDay(k + 1)
	}
	return disputedMonthStarts[monthStart] || disputedMonthStarts[getNewMoonDay(k+2)]
}

// canNames and chiNames are the sexagesimal-cycle stems and branches used to
// name lunar years (Giáp Thìn, Ất Tỵ, …).
var canNames = [...]string{"Giáp", "Ất", "Bính", "Đinh", "Mậu", "Kỷ", "Canh", "Tân", "Nhâm", "Quý"}
var chiNames = [...]string{"Tý", "Sửu", "Dần", "Mão", "Thìn", "Tỵ", "Ngọ", "Mùi", "Thân", "Dậu", "Tuất", "Hợi"}

// canChiYear returns the can-chi name of a lunar year, e.g. 2024 → "Giáp Thìn".
func canChiYear(year int) string {
	return canNames[(year+6)%10] + " " + chiNames[(year+8)%12]
}

// leapLabel renders the " nhuận" suffix appended to leap-month numbers.
func leapLabel(leap bool) string {
	if leap {
		return " nhuận"
	}
	return ""
}
