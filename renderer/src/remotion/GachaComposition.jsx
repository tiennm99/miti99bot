import {AbsoluteFill, Easing, interpolate, useCurrentFrame, useVideoConfig} from 'remotion';
import {
  createRandom,
  createStarfield,
  gachaTimeline,
  getEmblemGlyph,
  getLabelFontSize,
  getMeteorPoint,
  getRarityPalette,
  getStarRevealTimes,
  starColor,
} from './gacha-timeline.js';

const baseFont =
  'Quicksand, Inter, "Noto Sans", "Noto Sans Vietnamese", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif';

const clamp = /** @type {const} */ ({extrapolateLeft: 'clamp', extrapolateRight: 'clamp'});

const skyStars = createStarfield(70, 7);
const trailSamples = 34;
const trailSpacing = 0.014;

/**
 * Sparks shed by the meteor. Each is born at a point along the fall and
 * drifts away from it while fading.
 */
const meteorSparks = (() => {
  const random = createRandom(11);
  return Array.from({length: 40}, () => ({
    born: random(),
    driftX: (random() - 0.5) * 0.08,
    driftY: (random() - 0.2) * 0.08,
    size: 0.006 + random() * 0.01,
  }));
})();

/** Motes that float upward behind the revealed item. */
const revealMotes = (() => {
  const random = createRandom(23);
  return Array.from({length: 36}, () => ({
    x: random(),
    y: random(),
    speed: 0.03 + random() * 0.06,
    size: 0.004 + random() * 0.008,
    phase: random() * Math.PI * 2,
  }));
})();

/**
 * @param {string} hex
 * @param {number} alpha
 */
const withAlpha = (hex, alpha) => {
  const value = Number.parseInt(hex.slice(1), 16);
  return `rgba(${(value >> 16) & 255}, ${(value >> 8) & 255}, ${value & 255}, ${alpha})`;
};

/**
 * @param {{size: number}} props
 */
const StarIcon = ({size}) => (
  <svg height={size} viewBox="0 0 24 24" width={size}>
    <path
      d="M12 1.6l3.1 6.6 7.2.9-5.3 5 1.4 7.1L12 17.7l-6.4 3.5 1.4-7.1-5.3-5 7.2-.9z"
      fill={starColor}
      stroke="#fff6cf"
      strokeLinejoin="round"
      strokeWidth={0.8}
    />
  </svg>
);

/**
 * @typedef {import('../schemas/gacha-request.js').GachaRenderRequest} GachaRenderRequest
 */

/**
 * Wish animation: a meteor coloured by rarity falls across a night sky, lands
 * in a white flash, and the result is revealed with its stars popping in.
 *
 * @param {GachaRenderRequest} props
 */
export const GachaComposition = (props) => {
  const frame = useCurrentFrame();
  const {fps, width, height} = useVideoConfig();
  const seconds = frame / fps;
  const palette = getRarityPalette(props.rarity);
  const tl = gachaTimeline;

  const meteorProgress = interpolate(seconds, [tl.meteorStart, tl.meteorEnd], [0, 1], {
    ...clamp,
    easing: Easing.in(Easing.quad),
  });
  const meteorVisible = seconds >= tl.meteorStart && seconds < tl.flashHoldEnd;
  const head = getMeteorPoint(meteorProgress, width, height);
  const headRadius = height * interpolate(meteorProgress, [0, 1], [0.025, 0.085], clamp);

  const flashOpacity = interpolate(seconds, [tl.flashStart, tl.flashPeak, tl.flashHoldEnd, tl.flashEnd], [0, 1, 1, 0], {
    ...clamp,
    easing: Easing.inOut(Easing.quad),
  });
  const impact = getMeteorPoint(1, width, height);
  const bloomRadius = interpolate(seconds, [tl.flashStart, tl.flashPeak], [0.1, 2.2], clamp) * width;

  const revealed = seconds >= tl.revealStart;
  const emblemSize = height * 0.52;
  const emblemX = width * 0.66;
  const emblemY = height * 0.47;
  const emblemScale = interpolate(seconds, [tl.revealStart, tl.revealStart + 0.45, tl.emblemSettled], [0.72, 1.06, 1], {
    ...clamp,
    easing: Easing.out(Easing.cubic),
  });
  const silhouette = interpolate(seconds, [tl.revealStart + 0.2, tl.emblemSettled], [1, 0], clamp);
  const ringProgress = interpolate(seconds, [tl.revealStart, tl.revealStart + 0.9], [0, 1], {
    ...clamp,
    easing: Easing.out(Easing.quad),
  });
  const raysOpacity = interpolate(
    seconds,
    [tl.revealStart, tl.emblemSettled],
    [0, props.rarity === 3 ? 0.22 : 0.4],
    clamp,
  );
  const emblemPulse = 0.5 + 0.5 * Math.sin(seconds * 3);

  const nameProgress = interpolate(seconds, [tl.nameIn, tl.nameSettled], [0, 1], {
    ...clamp,
    easing: Easing.out(Easing.cubic),
  });
  const starSize = height * 0.09;
  const starTimes = getStarRevealTimes(props.rarity);

  return (
    <AbsoluteFill style={{background: '#05060f', fontFamily: baseFont, overflow: 'hidden'}}>
      {!revealed ? (
        <AbsoluteFill
          style={{
            background: 'linear-gradient(180deg, #070a24 0%, #121a4a 45%, #2a3a7c 80%, #3d4f96 100%)',
          }}
        >
          <AbsoluteFill
            style={{
              background: `radial-gradient(ellipse 80% 40% at 50% 100%, ${withAlpha('#9bb4ff', 0.35)} 0%, transparent 70%)`,
            }}
          />
          {skyStars.map((star, index) => (
            <div
              key={`sky-${index}`}
              style={{
                background: '#ffffff',
                borderRadius: '50%',
                height: star.radius * 2 * height,
                left: star.x * width,
                opacity: 0.35 + 0.65 * (0.5 + 0.5 * Math.sin(seconds * 2.4 + star.phase)),
                position: 'absolute',
                top: star.y * height,
                width: star.radius * 2 * height,
              }}
            />
          ))}

          {meteorVisible
            ? Array.from({length: trailSamples}, (_, index) => {
                const sampleProgress = meteorProgress - index * trailSpacing;
                if (sampleProgress <= 0) {
                  return null;
                }
                const point = getMeteorPoint(sampleProgress, width, height);
                const fade = 1 - index / trailSamples;
                const radius = headRadius * (0.25 + 0.75 * fade ** 1.3);
                return (
                  <div
                    key={`trail-${index}`}
                    style={{
                      background: `radial-gradient(circle, ${withAlpha(palette.glow, 0.85 * fade ** 1.4)} 0%, transparent 70%)`,
                      height: radius * 2,
                      left: point.x - radius,
                      position: 'absolute',
                      top: point.y - radius,
                      width: radius * 2,
                    }}
                  />
                );
              })
            : null}

          {meteorVisible
            ? meteorSparks.map((spark, index) => {
                if (spark.born > meteorProgress) {
                  return null;
                }
                const age = (meteorProgress - spark.born) * 4;
                if (age >= 1) {
                  return null;
                }
                const origin = getMeteorPoint(spark.born, width, height);
                const size = spark.size * height * (1 - age);
                return (
                  <div
                    key={`spark-${index}`}
                    style={{
                      background: palette.core,
                      borderRadius: '50%',
                      boxShadow: `0 0 ${size * 2}px ${palette.glow}`,
                      height: size,
                      left: origin.x + spark.driftX * width * age,
                      opacity: 1 - age,
                      position: 'absolute',
                      top: origin.y + spark.driftY * height * age,
                      width: size,
                    }}
                  />
                );
              })
            : null}

          {meteorVisible ? (
            <>
              <div
                style={{
                  background: `radial-gradient(circle, ${palette.core} 0%, ${palette.core} 18%, ${withAlpha(palette.glow, 0.9)} 34%, ${withAlpha(palette.glow, 0.35)} 58%, transparent 72%)`,
                  height: headRadius * 4,
                  left: head.x - headRadius * 2,
                  position: 'absolute',
                  top: head.y - headRadius * 2,
                  width: headRadius * 4,
                }}
              />
              {props.rarity === 5 ? (
                <>
                  <div
                    style={{
                      background: `linear-gradient(90deg, transparent, ${withAlpha(palette.core, 0.9)}, transparent)`,
                      height: Math.max(2, headRadius * 0.12),
                      left: head.x - headRadius * 5,
                      position: 'absolute',
                      top: head.y - Math.max(1, headRadius * 0.06),
                      width: headRadius * 10,
                    }}
                  />
                  <div
                    style={{
                      background: `linear-gradient(180deg, transparent, ${withAlpha(palette.core, 0.9)}, transparent)`,
                      height: headRadius * 6,
                      left: head.x - Math.max(1, headRadius * 0.06),
                      position: 'absolute',
                      top: head.y - headRadius * 3,
                      width: Math.max(2, headRadius * 0.12),
                    }}
                  />
                </>
              ) : null}
            </>
          ) : null}
        </AbsoluteFill>
      ) : null}

      {revealed ? (
        <AbsoluteFill
          style={{
            background: `radial-gradient(ellipse 70% 90% at 66% 47%, ${palette.deep} 0%, #0d0b1f 62%, #05060f 100%)`,
          }}
        >
          <div
            style={{
              background: `repeating-conic-gradient(from 0deg, ${withAlpha(palette.glow, 0.9)} 0deg 4deg, transparent 4deg 18deg)`,
              borderRadius: '50%',
              height: width * 1.6,
              left: emblemX - width * 0.8,
              maskImage: 'radial-gradient(circle, black 8%, transparent 55%)',
              opacity: raysOpacity,
              position: 'absolute',
              top: emblemY - width * 0.8,
              transform: `rotate(${seconds * 9}deg)`,
              WebkitMaskImage: 'radial-gradient(circle, black 8%, transparent 55%)',
              width: width * 1.6,
            }}
          />

          {revealMotes.map((mote, index) => {
            const y = (((mote.y - (seconds - tl.revealStart) * mote.speed) % 1) + 1) % 1;
            const size = mote.size * height;
            return (
              <div
                key={`mote-${index}`}
                style={{
                  background: palette.core,
                  borderRadius: '50%',
                  boxShadow: `0 0 ${size * 3}px ${palette.glow}`,
                  height: size,
                  left: mote.x * width,
                  opacity: 0.25 + 0.5 * (0.5 + 0.5 * Math.sin(seconds * 3 + mote.phase)),
                  position: 'absolute',
                  top: y * height,
                  width: size,
                }}
              />
            );
          })}

          {ringProgress < 1 ? (
            <div
              style={{
                border: `${Math.max(2, height * 0.008)}px solid ${palette.glow}`,
                borderRadius: '50%',
                boxShadow: `0 0 ${height * 0.04}px ${palette.glow}, inset 0 0 ${height * 0.04}px ${palette.glow}`,
                height: emblemSize,
                left: emblemX - emblemSize / 2,
                opacity: 1 - ringProgress,
                position: 'absolute',
                top: emblemY - emblemSize / 2,
                transform: `scale(${0.4 + ringProgress * 2.6})`,
                width: emblemSize,
              }}
            />
          ) : null}

          <div
            style={{
              height: emblemSize,
              left: emblemX - emblemSize / 2,
              position: 'absolute',
              top: emblemY - emblemSize / 2,
              transform: `scale(${emblemScale})`,
              width: emblemSize,
            }}
          >
            <div
              style={{
                background: `linear-gradient(135deg, ${palette.emblem})`,
                border: `${Math.max(2, height * 0.008)}px solid rgba(255, 255, 255, 0.85)`,
                borderRadius: emblemSize * 0.12,
                boxShadow: `0 0 ${height * (0.05 + 0.03 * emblemPulse)}px ${palette.glow}, inset 0 0 ${height * 0.04}px rgba(255, 255, 255, 0.55)`,
                height: emblemSize * 0.72,
                left: emblemSize * 0.14,
                overflow: 'hidden',
                position: 'absolute',
                top: emblemSize * 0.14,
                transform: 'rotate(45deg)',
                width: emblemSize * 0.72,
              }}
            >
              <div
                style={{
                  background: 'linear-gradient(135deg, rgba(255,255,255,0.55) 0%, rgba(255,255,255,0) 45%)',
                  inset: 0,
                  position: 'absolute',
                }}
              />
              <div style={{background: '#ffffff', inset: 0, opacity: silhouette, position: 'absolute'}} />
            </div>
            <div
              style={{
                alignItems: 'center',
                color: '#ffffff',
                display: 'flex',
                fontSize: emblemSize * 0.36,
                fontWeight: 800,
                inset: 0,
                justifyContent: 'center',
                opacity: 1 - silhouette,
                position: 'absolute',
                textShadow: `0 2px ${height * 0.01}px rgba(0, 0, 0, 0.45), 0 0 ${height * 0.03}px ${palette.glow}`,
              }}
            >
              {getEmblemGlyph(props.label)}
            </div>
          </div>

          <div
            style={{
              left: width * 0.07,
              opacity: nameProgress,
              position: 'absolute',
              top: height * 0.56,
              transform: `translateX(${(1 - nameProgress) * -width * 0.06}px)`,
              width: width * 0.44,
            }}
          >
            <div
              style={{
                color: '#ffffff',
                display: '-webkit-box',
                fontSize: getLabelFontSize(props.label, height),
                fontWeight: 800,
                lineHeight: 1.12,
                overflow: 'hidden',
                overflowWrap: 'anywhere',
                textShadow: `0 2px ${height * 0.012}px rgba(0, 0, 0, 0.7), 0 0 ${height * 0.035}px ${withAlpha(palette.glow, 0.8)}`,
                WebkitBoxOrient: 'vertical',
                WebkitLineClamp: 2,
              }}
            >
              {props.label}
            </div>
            <div style={{display: 'flex', gap: starSize * 0.12, marginTop: height * 0.025}}>
              {starTimes.map((time, index) => {
                const pop = interpolate(seconds, [time, time + 0.12, time + 0.24], [0, 1.6, 1], clamp);
                const flare = interpolate(seconds, [time, time + 0.3], [1, 0], clamp);
                return (
                  <div
                    key={`star-${index}`}
                    style={{
                      height: starSize,
                      opacity: seconds >= time ? 1 : 0,
                      position: 'relative',
                      transform: `scale(${pop})`,
                      width: starSize,
                    }}
                  >
                    <div
                      style={{
                        background: `radial-gradient(circle, ${withAlpha('#fff4c2', 0.9 * flare)} 0%, transparent 65%)`,
                        height: starSize * 2.4,
                        left: -starSize * 0.7,
                        position: 'absolute',
                        top: -starSize * 0.7,
                        width: starSize * 2.4,
                      }}
                    />
                    <StarIcon size={starSize} />
                  </div>
                );
              })}
            </div>
          </div>
        </AbsoluteFill>
      ) : null}

      {flashOpacity > 0 ? (
        <AbsoluteFill style={{pointerEvents: 'none'}}>
          <div
            style={{
              background: `radial-gradient(circle, #ffffff 0%, #ffffff 35%, ${withAlpha(palette.core, 0)} 70%)`,
              height: bloomRadius * 2,
              left: impact.x - bloomRadius,
              opacity: Math.min(1, flashOpacity * 1.6),
              position: 'absolute',
              top: impact.y - bloomRadius,
              width: bloomRadius * 2,
            }}
          />
          <AbsoluteFill style={{background: '#ffffff', opacity: flashOpacity}} />
        </AbsoluteFill>
      ) : null}
    </AbsoluteFill>
  );
};
