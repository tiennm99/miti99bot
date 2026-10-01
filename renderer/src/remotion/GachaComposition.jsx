import {AbsoluteFill, Easing, interpolate, useCurrentFrame, useVideoConfig} from 'remotion';
import {
  createRandom,
  createStarfield,
  gachaTimeline,
  getLabelFontSize,
  getMeteorState,
  getRankLetter,
  getRarityPalette,
  getShakeOffset,
  getStarRevealTimes,
  getTierEffects,
  starColor,
} from './gacha-timeline.js';

const baseFont =
  'Quicksand, Inter, "Noto Sans", "Noto Sans Vietnamese", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif';

const clamp = /** @type {const} */ ({extrapolateLeft: 'clamp', extrapolateRight: 'clamp'});

const skyStars = createStarfield(70, 7);
/** Seconds between motion-streak samples; the streak is the meteor's recent path. */
const trailStep = 0.016;
const sparkLifetime = 0.55;
const meteorDuration = gachaTimeline.meteorEnd - gachaTimeline.meteorStart;
const starburstSpikes = 10;

/**
 * Sparks shed by the meteor. Each is born on the path, keeps part of the
 * meteor's velocity plus a random kick, and then falls under gravity while
 * fading. Tiers draw a prefix of this pool.
 */
const meteorSparks = (() => {
  const random = createRandom(11);
  return Array.from({length: 80}, () => ({
    born: random() * meteorDuration,
    inherit: 0.15 + random() * 0.3,
    kickX: (random() - 0.5) * 0.5,
    kickY: (random() - 0.7) * 0.5,
    size: 0.006 + random() * 0.012,
  }));
})();

/** Motes that float upward behind the revealed item. */
const revealMotes = (() => {
  const random = createRandom(23);
  return Array.from({length: 56}, () => ({
    x: random(),
    y: random(),
    speed: 0.03 + random() * 0.07,
    size: 0.004 + random() * 0.009,
    phase: random() * Math.PI * 2,
  }));
})();

/** Sparkles that rain down across the 5★ reveal. */
const rainSparkles = (() => {
  const random = createRandom(31);
  return Array.from({length: 40}, () => ({
    x: random(),
    offset: random(),
    speed: 0.25 + random() * 0.35,
    size: 0.025 + random() * 0.03,
    spin: (random() - 0.5) * 360,
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
 * in a white flash, and the result is revealed with its rank letter and stars.
 * Every beat scales with the tier's effects so higher rarities are louder.
 *
 * @param {GachaRenderRequest} props
 */
export const GachaComposition = (props) => {
  const frame = useCurrentFrame();
  const {fps, width, height} = useVideoConfig();
  const seconds = frame / fps;
  const palette = getRarityPalette(props.rarity);
  const fx = getTierEffects(props.rarity);
  const tl = gachaTimeline;

  const meteorElapsed = seconds - tl.meteorStart;
  const meteorProgress = Math.min(1, Math.max(0, meteorElapsed / meteorDuration));
  const meteorVisible = seconds >= tl.meteorStart && seconds < tl.flashHoldEnd;
  const head = getMeteorState(meteorElapsed, width, height);
  const headRadius = height * interpolate(meteorProgress, [0, 1], [0.025, 0.085], clamp) * fx.headScale;
  // Stretch the head along its velocity like motion blur, keeping its area.
  const headSpeed = Math.hypot(head.vx, head.vy);
  const headStretch = 1 + Math.min(0.9, headSpeed / (height * 1.6));
  const headAngle = (Math.atan2(head.vy, head.vx) * 180) / Math.PI;
  const haloOpacity = fx.halo ? interpolate(meteorProgress, [0.45, 0.8], [0, 1], clamp) : 0;
  const skyFlood = interpolate(seconds, [tl.meteorStart + 1.2, tl.meteorEnd], [0, fx.skyFlood], clamp);

  const flashOpacity = interpolate(seconds, [tl.flashStart, tl.flashPeak, tl.flashHoldEnd, tl.flashEnd], [0, 1, 1, 0], {
    ...clamp,
    easing: Easing.inOut(Easing.quad),
  });
  const impact = getMeteorState(meteorDuration, width, height);
  const bloomRadius = interpolate(seconds, [tl.flashStart, tl.flashPeak], [0.1, 2.2], clamp) * width;

  const revealed = seconds >= tl.revealStart;
  const shake = getShakeOffset(seconds, tl.revealStart, fx.shake * height);
  const emblemSize = height * 0.52;
  const emblemX = width * 0.66;
  const emblemY = height * 0.47;
  const emblemScale = interpolate(
    seconds,
    [tl.revealStart, tl.revealStart + 0.45, tl.emblemSettled],
    [0.72, 1 + 0.04 * fx.headScale, 1],
    {...clamp, easing: Easing.out(Easing.cubic)},
  );
  const silhouette = interpolate(seconds, [tl.revealStart + 0.2, tl.emblemSettled], [1, 0], clamp);
  const raysOpacity = interpolate(seconds, [tl.revealStart, tl.emblemSettled], [0, fx.raysOpacity], clamp);
  const starburstScale = interpolate(seconds, [tl.revealStart, tl.emblemSettled + 0.3], [0.2, 1], {
    ...clamp,
    easing: Easing.out(Easing.cubic),
  });
  const emblemPulse = 0.5 + 0.5 * Math.sin(seconds * 3);
  const sheenProgress = ((((seconds - tl.emblemSettled) / 1.8) % 1) + 1) % 1;
  const rankLetter = getRankLetter(props.rarity);
  const rankLetterStyle =
    props.rarity === 5
      ? {
          background: 'linear-gradient(180deg, #ffffff 0%, #fff6d0 55%, #ffd36b 100%)',
          color: 'transparent',
          filter: `drop-shadow(0 0 ${height * 0.006}px #8a4b00) drop-shadow(0 0 ${height * 0.006}px #8a4b00) drop-shadow(0 0 ${height * 0.03}px ${palette.glow})`,
          WebkitBackgroundClip: 'text',
          backgroundClip: 'text',
        }
      : {
          color: '#ffffff',
          textShadow: `0 2px ${height * 0.01}px rgba(0, 0, 0, 0.45), 0 0 ${height * 0.03}px ${palette.glow}`,
        };

  const nameProgress = interpolate(seconds, [tl.nameIn, tl.nameSettled], [0, 1], {
    ...clamp,
    easing: Easing.out(Easing.cubic),
  });
  const starSize = height * 0.09;
  const starTimes = getStarRevealTimes(props.rarity);
  const rayMask = 'radial-gradient(circle, black 8%, transparent 55%)';

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

          {skyFlood > 0 ? (
            <AbsoluteFill
              style={{
                background: `radial-gradient(circle at ${(impact.x / width) * 100}% ${(impact.y / height) * 100}%, ${withAlpha(palette.glow, 0.95)} 0%, ${withAlpha(palette.glow, 0.4)} 40%, transparent 80%)`,
                mixBlendMode: 'screen',
                opacity: skyFlood,
              }}
            />
          ) : null}

          {meteorVisible
            ? Array.from({length: fx.trailSamples}, (_, index) => {
                const sampleElapsed = Math.min(meteorElapsed, meteorDuration) - index * trailStep;
                if (sampleElapsed <= 0) {
                  return null;
                }
                const point = getMeteorState(sampleElapsed, width, height);
                const fade = 1 - index / fx.trailSamples;
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
            ? meteorSparks.slice(0, fx.sparks).map((spark, index) => {
                const age = Math.min(meteorElapsed, meteorDuration) - spark.born;
                if (age < 0 || age >= sparkLifetime) {
                  return null;
                }
                const origin = getMeteorState(spark.born, width, height);
                const life = age / sparkLifetime;
                const size = spark.size * height * (1 - life) * fx.headScale;
                const sparkGravity = height * 0.9;
                const x = origin.x + (origin.vx * spark.inherit + spark.kickX * height) * age;
                const y =
                  origin.y + (origin.vy * spark.inherit + spark.kickY * height) * age + 0.5 * sparkGravity * age * age;
                return (
                  <div
                    key={`spark-${index}`}
                    style={{
                      background: palette.core,
                      borderRadius: '50%',
                      boxShadow: `0 0 ${size * 2}px ${palette.glow}`,
                      height: size,
                      left: x,
                      opacity: 1 - life,
                      position: 'absolute',
                      top: y,
                      width: size,
                    }}
                  />
                );
              })
            : null}

          {meteorVisible && haloOpacity > 0 ? (
            <div
              style={{
                background: 'conic-gradient(from 0deg, #ff6b6b, #ffd36b, #8cff9a, #6bd6ff, #b48cff, #ff8cd8, #ff6b6b)',
                borderRadius: '50%',
                height: headRadius * 6,
                left: head.x - headRadius * 3,
                maskImage: 'radial-gradient(circle, transparent 52%, black 58%, black 64%, transparent 72%)',
                opacity: haloOpacity * 0.9,
                position: 'absolute',
                top: head.y - headRadius * 3,
                transform: `rotate(${seconds * 240}deg) scale(${0.7 + 0.3 * haloOpacity})`,
                WebkitMaskImage: 'radial-gradient(circle, transparent 52%, black 58%, black 64%, transparent 72%)',
                width: headRadius * 6,
              }}
            />
          ) : null}

          {meteorVisible ? (
            <>
              <div
                style={{
                  background: `radial-gradient(circle, ${palette.core} 0%, ${palette.core} 18%, ${withAlpha(palette.glow, 0.9)} 34%, ${withAlpha(palette.glow, 0.35)} 58%, transparent 72%)`,
                  height: headRadius * 4,
                  left: head.x - headRadius * 2,
                  position: 'absolute',
                  top: head.y - headRadius * 2,
                  transform: `rotate(${headAngle}deg) scale(${headStretch}, ${1 / Math.sqrt(headStretch)})`,
                  width: headRadius * 4,
                }}
              />
              {props.rarity >= 4 ? (
                <>
                  <div
                    style={{
                      background: `linear-gradient(90deg, transparent, ${withAlpha(palette.core, 0.9)}, transparent)`,
                      height: Math.max(2, headRadius * 0.12),
                      left: head.x - headRadius * (props.rarity === 5 ? 6 : 3.5),
                      position: 'absolute',
                      top: head.y - Math.max(1, headRadius * 0.06),
                      width: headRadius * (props.rarity === 5 ? 12 : 7),
                    }}
                  />
                  <div
                    style={{
                      background: `linear-gradient(180deg, transparent, ${withAlpha(palette.core, 0.9)}, transparent)`,
                      height: headRadius * (props.rarity === 5 ? 8 : 4.5),
                      left: head.x - Math.max(1, headRadius * 0.06),
                      position: 'absolute',
                      top: head.y - headRadius * (props.rarity === 5 ? 4 : 2.25),
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
            transform: `translate(${shake.x}px, ${shake.y}px) scale(1.04)`,
          }}
        >
          <div
            style={{
              background: `repeating-conic-gradient(from 0deg, ${withAlpha(palette.glow, 0.9)} 0deg 4deg, transparent 4deg 18deg)`,
              borderRadius: '50%',
              height: width * 1.6,
              left: emblemX - width * 0.8,
              maskImage: rayMask,
              opacity: raysOpacity,
              position: 'absolute',
              top: emblemY - width * 0.8,
              transform: `rotate(${seconds * 9}deg)`,
              WebkitMaskImage: rayMask,
              width: width * 1.6,
            }}
          />
          {fx.counterRays ? (
            <div
              style={{
                background: `repeating-conic-gradient(from 9deg, ${withAlpha(palette.core, 0.8)} 0deg 2deg, transparent 2deg 24deg)`,
                borderRadius: '50%',
                height: width * 1.6,
                left: emblemX - width * 0.8,
                maskImage: rayMask,
                opacity: raysOpacity * 0.8,
                position: 'absolute',
                top: emblemY - width * 0.8,
                transform: `rotate(${-seconds * 14}deg)`,
                WebkitMaskImage: rayMask,
                width: width * 1.6,
              }}
            />
          ) : null}

          {fx.starburst
            ? Array.from({length: starburstSpikes}, (_, index) => {
                const long = index % 2 === 0;
                const length = height * (long ? 1.1 : 0.7) * starburstScale;
                return (
                  <div
                    key={`spike-${index}`}
                    style={{
                      background: `linear-gradient(90deg, ${withAlpha(palette.core, 0.95)}, ${withAlpha(palette.glow, 0.5)} 40%, transparent)`,
                      height: Math.max(2, height * (long ? 0.012 : 0.007)),
                      left: emblemX,
                      position: 'absolute',
                      top: emblemY,
                      transform: `rotate(${(360 / starburstSpikes) * index + seconds * 6}deg)`,
                      transformOrigin: '0 50%',
                      width: length,
                    }}
                  />
                );
              })
            : null}

          {revealMotes.slice(0, fx.motes).map((mote, index) => {
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

          {rainSparkles.slice(0, fx.sparkleRain).map((sparkle, index) => {
            const y = ((sparkle.offset + (seconds - tl.revealStart) * sparkle.speed) % 1.2) - 0.1;
            const size = sparkle.size * height;
            return (
              <div
                key={`rain-${index}`}
                style={{
                  background: `linear-gradient(135deg, #fffbe6, ${palette.glow})`,
                  boxShadow: `0 0 ${size}px ${palette.glow}`,
                  height: size,
                  left: sparkle.x * width,
                  opacity: 0.85,
                  position: 'absolute',
                  top: y * height,
                  clipPath: 'polygon(50% 0%, 61% 39%, 100% 50%, 61% 61%, 50% 100%, 39% 61%, 0% 50%, 39% 39%)',
                  transform: `rotate(${seconds * sparkle.spin}deg)`,
                  width: size,
                }}
              />
            );
          })}

          {Array.from({length: fx.shockwaves}, (_, index) => {
            const start = tl.revealStart + index * 0.16;
            const progress = interpolate(seconds, [start, start + 0.9], [0, 1], {
              ...clamp,
              easing: Easing.out(Easing.quad),
            });
            if (progress <= 0 || progress >= 1) {
              return null;
            }
            return (
              <div
                key={`wave-${index}`}
                style={{
                  border: `${Math.max(2, height * (0.01 - index * 0.002))}px solid ${index === 0 ? palette.glow : palette.core}`,
                  borderRadius: '50%',
                  boxShadow: `0 0 ${height * 0.04}px ${palette.glow}, inset 0 0 ${height * 0.04}px ${palette.glow}`,
                  height: emblemSize,
                  left: emblemX - emblemSize / 2,
                  opacity: 1 - progress,
                  position: 'absolute',
                  top: emblemY - emblemSize / 2,
                  transform: `scale(${0.4 + progress * (2.4 + index * 0.8)})`,
                  width: emblemSize,
                }}
              />
            );
          })}

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
                border: `${Math.max(2, height * (props.rarity === 5 ? 0.014 : 0.008))}px solid ${props.rarity === 5 ? '#fff0b8' : 'rgba(255, 255, 255, 0.85)'}`,
                borderRadius: emblemSize * 0.12,
                boxShadow: `0 0 ${height * (0.04 + 0.03 * fx.headScale * emblemPulse)}px ${palette.glow}, inset 0 0 ${height * 0.04}px rgba(255, 255, 255, 0.55)`,
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
              {fx.sheen && seconds >= tl.emblemSettled ? (
                <div
                  style={{
                    background: 'linear-gradient(90deg, transparent, rgba(255,255,255,0.75), transparent)',
                    height: '100%',
                    left: `${-60 + sheenProgress * 220}%`,
                    position: 'absolute',
                    top: 0,
                    width: '40%',
                  }}
                />
              ) : null}
              <div style={{background: '#ffffff', inset: 0, opacity: silhouette, position: 'absolute'}} />
            </div>
            <div
              style={{
                alignItems: 'center',
                display: 'flex',
                fontSize: emblemSize * 0.42,
                fontStyle: 'italic',
                fontWeight: 900,
                inset: 0,
                justifyContent: 'center',
                opacity: 1 - silhouette,
                position: 'absolute',
              }}
            >
              <span style={{paddingRight: emblemSize * 0.03, ...rankLetterStyle}}>{rankLetter}</span>
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
                const flareSize = starSize * 2.4 * fx.starFlare;
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
                        height: flareSize,
                        left: (starSize - flareSize) / 2,
                        position: 'absolute',
                        top: (starSize - flareSize) / 2,
                        width: flareSize,
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
          <AbsoluteFill style={{background: props.rarity === 5 ? '#fff6dc' : '#ffffff', opacity: flashOpacity}} />
        </AbsoluteFill>
      ) : null}
    </AbsoluteFill>
  );
};
