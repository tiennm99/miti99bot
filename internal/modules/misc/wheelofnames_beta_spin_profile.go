package misc

import (
	"math"
	"math/rand/v2"
)

const (
	wheelBetaMinRevolutions        = 7
	wheelBetaRandomRevolutionRange = 5
	wheelBetaMaxLandingOffset      = 0.34
)

type wheelBetaSpinProfile struct {
	startRotation   float64
	finalRotation   float64
	accelEnd        float64
	coastEnd        float64
	wobbleAmplitude float64
	wobbleCycles    float64
	wobblePhase     float64
}

func newWheelBetaSpinProfile(optionCount, winner int, rng *rand.Rand) wheelBetaSpinProfile {
	segment := 2 * math.Pi / float64(optionCount)
	landingOffset := (wheelBetaRandFloat64(rng)*2 - 1) * wheelBetaMaxLandingOffset
	finalRotation := finalWheelRotationWithOffset(optionCount, winner, landingOffset)
	revolutions := wheelBetaMinRevolutions + wheelBetaRandIntN(rng, wheelBetaRandomRevolutionRange)
	accelEnd := 0.12 + wheelBetaRandFloat64(rng)*0.1
	coastEnd := accelEnd + 0.18 + wheelBetaRandFloat64(rng)*0.18
	if coastEnd > 0.58 {
		coastEnd = 0.58
	}
	return wheelBetaSpinProfile{
		startRotation:   finalRotation - float64(revolutions)*2*math.Pi,
		finalRotation:   finalRotation,
		accelEnd:        accelEnd,
		coastEnd:        coastEnd,
		wobbleAmplitude: math.Min(segment*0.075, 0.09) * (0.55 + wheelBetaRandFloat64(rng)*0.45),
		wobbleCycles:    3.5 + wheelBetaRandFloat64(rng)*2.5,
		wobblePhase:     wheelBetaRandFloat64(rng) * 2 * math.Pi,
	}
}

func (p wheelBetaSpinProfile) rotationAt(t float64) float64 {
	t = clampWheelBetaProgress(t)
	progress := p.progressAt(t)
	rotation := p.startRotation + (p.finalRotation-p.startRotation)*progress
	return rotation + p.wobbleAt(t)
}

func (p wheelBetaSpinProfile) statusAt(t float64) string {
	switch {
	case t < p.accelEnd:
		return "BUILDING SPEED"
	case t < p.coastEnd:
		return "FULL SPIN"
	case t < 0.82:
		return "HEAVY SLOWDOWN"
	case t < 0.96:
		return "LOCKING IN"
	default:
		return "LAST TICK"
	}
}

func (p wheelBetaSpinProfile) progressAt(t float64) float64 {
	t = clampWheelBetaProgress(t)
	accelEnd := p.accelEnd
	coastEnd := p.coastEnd
	if accelEnd <= 0 || coastEnd <= accelEnd || coastEnd >= 1 {
		remaining := 1 - t
		return 1 - remaining*remaining*remaining
	}

	accelArea := accelEnd * 0.5
	coastArea := coastEnd - accelEnd
	decelDuration := 1 - coastEnd
	decelArea := decelDuration * 0.25
	totalArea := accelArea + coastArea + decelArea

	var distance float64
	switch {
	case t < accelEnd:
		u := t / accelEnd
		distance = accelEnd * (u*u*u - 0.5*u*u*u*u)
	case t < coastEnd:
		distance = accelArea + (t - accelEnd)
	default:
		u := (t - coastEnd) / decelDuration
		distance = accelArea + coastArea + decelDuration*(u-1.5*u*u+u*u*u-0.25*u*u*u*u)
	}
	return distance / totalArea
}

func (p wheelBetaSpinProfile) wobbleAt(t float64) float64 {
	if t <= p.coastEnd || t >= 1 {
		return 0
	}
	u := (t - p.coastEnd) / (1 - p.coastEnd)
	envelope := math.Sin(u*math.Pi) * math.Pow(1-u, 1.4)
	return p.wobbleAmplitude * envelope * math.Sin(p.wobblePhase+u*p.wobbleCycles*2*math.Pi)
}

func clampWheelBetaProgress(t float64) float64 {
	switch {
	case t < 0:
		return 0
	case t > 1:
		return 1
	default:
		return t
	}
}

func wheelBetaRandFloat64(rng *rand.Rand) float64 {
	if rng != nil {
		return rng.Float64()
	}
	return rand.Float64()
}

func wheelBetaRandIntN(rng *rand.Rand, n int) int {
	if n <= 0 {
		return 0
	}
	if rng != nil {
		return rng.IntN(n)
	}
	return rand.IntN(n)
}
