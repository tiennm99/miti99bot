package misc

import (
	"math"
	"math/rand/v2"
)

const (
	wheelMinRevolutions        = 7
	wheelRandomRevolutionRange = 5
	wheelMaxLandingOffset      = 0.34
)

type wheelSpinProfile struct {
	startRotation   float64
	finalRotation   float64
	accelEnd        float64
	decelSharpness  float64
	wobbleAmplitude float64
	wobbleCycles    float64
	wobblePhase     float64
}

func newWheelSpinProfile(optionCount, winner int, rng *rand.Rand) wheelSpinProfile {
	segment := 2 * math.Pi / float64(optionCount)
	landingOffset := (wheelRandFloat64(rng)*2 - 1) * wheelMaxLandingOffset
	finalRotation := finalWheelRotationWithOffset(optionCount, winner, landingOffset)
	revolutions := wheelMinRevolutions + wheelRandIntN(rng, wheelRandomRevolutionRange)
	accelEnd := 0.20 + wheelRandFloat64(rng)*0.1
	return wheelSpinProfile{
		startRotation:   finalRotation - float64(revolutions)*2*math.Pi,
		finalRotation:   finalRotation,
		accelEnd:        accelEnd,
		decelSharpness:  3.6 + wheelRandFloat64(rng)*1.8,
		wobbleAmplitude: math.Min(segment*0.075, 0.09) * (0.55 + wheelRandFloat64(rng)*0.45),
		wobbleCycles:    3.5 + wheelRandFloat64(rng)*2.5,
		wobblePhase:     wheelRandFloat64(rng) * 2 * math.Pi,
	}
}

func (p wheelSpinProfile) rotationAt(t float64) float64 {
	t = clampWheelProgress(t)
	progress := p.progressAt(t)
	rotation := p.startRotation + (p.finalRotation-p.startRotation)*progress
	return rotation + p.wobbleAt(t)
}

func (p wheelSpinProfile) statusAt(t float64) string {
	switch {
	case t < p.accelEnd:
		return "BUILDING SPEED"
	case t < 0.82:
		return "DECELERATING"
	case t < 0.96:
		return "LOCKING IN"
	default:
		return "LAST TICK"
	}
}

func (p wheelSpinProfile) progressAt(t float64) float64 {
	t = clampWheelProgress(t)
	if t == 0 || t == 1 {
		return t
	}

	accelEnd := p.accelEnd
	if accelEnd <= 0 || accelEnd >= 1 {
		remaining := 1 - t
		return 1 - remaining*remaining*remaining
	}

	accelDistance := accelEnd * 0.72
	if t < accelEnd {
		u := t / accelEnd
		return accelDistance * u * u
	}

	sharpness := p.decelSharpness
	if sharpness <= 0 {
		sharpness = 4
	}
	u := (t - accelEnd) / (1 - accelEnd)
	decelProgress := (1 - math.Exp(-sharpness*u)) / (1 - math.Exp(-sharpness))
	return accelDistance + (1-accelDistance)*decelProgress
}

func (p wheelSpinProfile) wobbleAt(t float64) float64 {
	if t <= p.accelEnd || t >= 1 {
		return 0
	}
	u := (t - p.accelEnd) / (1 - p.accelEnd)
	envelope := math.Sin(u*math.Pi) * math.Pow(1-u, 1.4)
	return p.wobbleAmplitude * envelope * math.Sin(p.wobblePhase+u*p.wobbleCycles*2*math.Pi)
}

func clampWheelProgress(t float64) float64 {
	switch {
	case t < 0:
		return 0
	case t > 1:
		return 1
	default:
		return t
	}
}

func wheelRandFloat64(rng *rand.Rand) float64 {
	if rng != nil {
		return rng.Float64()
	}
	return rand.Float64()
}

func wheelRandIntN(rng *rand.Rand, n int) int {
	if n <= 0 {
		return 0
	}
	if rng != nil {
		return rng.IntN(n)
	}
	return rand.IntN(n)
}
