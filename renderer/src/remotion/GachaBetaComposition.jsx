import {useMemo} from 'react';
import {Star} from '@remotion/shapes';
import {AbsoluteFill, Easing, Img, interpolate, useCurrentFrame, useVideoConfig} from 'remotion';
import {bakeCloudLayer} from './gacha-beta-paint.js';
import {
  createShards,
  gachaBetaTimeline,
  getBetaRank,
  getBurstState,
  getFlash,
  getHaloState,
  getMeteorHead,
  getRedMeteor,
  getShardOffset,
  getShot,
  getStarRevealTimes,
  getVolleyCometHead,
  getVortexState,
  progress,
  volleyComets,
} from './gacha-beta-scene.js';
import {createRandom, createStarfield, getRarityPalette, getTwinkle, starColor} from './gacha-timeline.js';

const sansFont =
  'Quicksand, Inter, "Noto Sans", "Noto Sans Vietnamese", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif';
const serifFont = '"Noto Serif", Georgia, "Times New Roman", serif';

const clamp = /** @type {const} */ ({
  extrapolateLeft: 'clamp',
  extrapolateRight: 'clamp',
});

/**
 * @param {string} hex
 * @param {number} alpha
 */
const withAlpha = (hex, alpha) => {
  const value = Number.parseInt(hex.slice(1), 16);
  return `rgba(${(value >> 16) & 255}, ${(value >> 8) & 255}, ${value & 255}, ${alpha})`;
};

/**
 * @param {number} value
 * @returns {number}
 */
const clamp01 = (value) => Math.min(1, Math.max(0, value));

const rainbow =
  'circle closest-side, transparent 0%, transparent 78%, rgba(255,70,70,0.75) 80%, rgba(255,170,40,0.75) 83%, rgba(255,240,80,0.7) 86%, rgba(80,230,120,0.7) 89%, rgba(70,160,255,0.75) 92%, rgba(150,90,255,0.7) 95%, transparent 98%';
const confettiColors = ['#ffb36b', '#ff7fb8', '#9ff0ff', '#fff2a0', '#c9a0ff'];

/** @typedef {import('./gacha-beta-paint.js').Puff} Puff */

/** Half the side of the baked vortex ring, in frame heights; it spans past the frame so it can rotate. */
const ringReach = 1.45;

/** The cloud banks only fill the lower frame, and are baked wider than it so they can drift. */
const cloudBandTop = 0.42;
const cloudBandHeight = 0.62;
const cloudBandWidth = 1.2;

/**
 * @param {() => number} random
 * @param {number} count
 * @returns {Puff[]}
 */
const createPuffs = (random, count) =>
  Array.from({length: count}, (_, index) => ({
    dx: (index / (count - 1) - 0.5) * 1.6 + (random() - 0.5) * 0.2,
    dy: -Math.sin((index / (count - 1)) * Math.PI) * 0.35 + (random() - 0.5) * 0.12,
    r: 0.3 + random() * 0.28,
  }));

/**
 * Every seeded layout for one roll, so each wish draws a different sky. The
 * cloud layers are baked here once and only moved per frame.
 *
 * @param {number} seed
 * @param {number} width
 * @param {number} height
 */
const createBetaLayout = (seed, width, height) => {
  const random = createRandom(seed + 307);
  const reach = ringReach * height;
  const ringClusters = Array.from({length: 22}, (_, index) => {
    const radians = (((index / 22) * 360 + random() * 12) * Math.PI) / 180;
    const distance = (0.5 + random() * 0.22) * height;
    return {
      x: reach + Math.cos(radians) * distance * 1.25,
      y: reach + Math.sin(radians) * distance,
      size: (0.22 + random() * 0.16) * height,
      puffs: createPuffs(random, 7),
    };
  });
  const bankClusters = Array.from({length: 19}, (_, index) => {
    const x = (index / 18) * cloudBandWidth;
    return {
      x: x * width,
      y: (0.93 - Math.max(0, Math.min(x, 1) - 0.55) * 0.55 - cloudBandTop) * height,
      size: (0.16 + random() * 0.12) * height,
      puffs: createPuffs(random, 7),
    };
  });
  const paint = {seed: seed + 41, frequency: 18 / height, strength: height * 0.022};
  const slivers = Array.from({length: 22}, () => ({
    along: random(),
    side: (random() - 0.5) * 2,
    length: 0.03 + random() * 0.07,
    phase: random(),
  }));
  const confetti = Array.from({length: 40}, (_, index) => ({
    angle: 200 + random() * 120,
    reach: 0.15 + random() * 0.45,
    size: 0.006 + random() * 0.01,
    delay: random() * 0.5,
    color: confettiColors[index % confettiColors.length] ?? '#ffffff',
  }));
  const shards = createShards(random, 34);
  const chains = Array.from({length: 5}, () => ({
    x: 0.3 + random() * 0.7,
    y: random() * 0.5,
    angle: -50 + random() * 100,
    count: 6 + Math.floor(random() * 8),
    drift: (random() - 0.5) * 0.04,
  }));
  const panels = Array.from({length: 9}, () => ({
    x: 0.25 + random() * 0.75,
    y: random() * 0.85,
    w: 0.02 + random() * 0.05,
    h: 0.03 + random() * 0.07,
    tilt: (random() - 0.5) * 40,
    phase: random() * Math.PI * 2,
  }));
  return {
    ringImage: bakeCloudLayer({...paint, width: reach * 2, height: reach * 2, clusters: ringClusters}),
    bankImage: bakeCloudLayer({
      ...paint,
      width: width * cloudBandWidth,
      height: height * cloudBandHeight,
      clusters: bankClusters,
    }),
    slivers,
    confetti,
    shards,
    chains,
    panels,
    stars: createStarfield(60, seed),
  };
};

/** @typedef {ReturnType<typeof createBetaLayout>} BetaLayout */
/** @typedef {{seconds: number, width: number, height: number, layout: BetaLayout, palette: import('./gacha-timeline.js').RarityPalette}} ShotProps */

/**
 * Four-point star glint with a soft core, used for meteor heads.
 *
 * @param {{x: number, y: number, size: number, color: string, core?: string, rays?: number, spin?: number}} props
 */
const Glint = ({x, y, size, color, core = '#ffffff', rays = 4, spin = 0}) => (
  <>
    <div
      style={{
        background: `radial-gradient(circle, ${core} 0%, ${withAlpha(color, 0.8)} 22%, ${withAlpha(color, 0.25)} 48%, transparent 70%)`,
        borderRadius: '50%',
        height: size,
        left: x - size / 2,
        position: 'absolute',
        top: y - size / 2,
        width: size,
      }}
    />
    {Array.from({length: rays}, (_, index) => {
      const long = index % 2 === 0 ? 1.6 : 0.9;
      return (
        <div
          key={index}
          style={{
            background: `linear-gradient(90deg, transparent, ${core} 50%, transparent)`,
            height: Math.max(1.5, size * 0.03),
            left: x - (size * long) / 2,
            position: 'absolute',
            top: y - Math.max(1.5, size * 0.03) / 2,
            transform: `rotate(${(index * 180) / rays + spin}deg)`,
            width: size * long,
          }}
        />
      );
    })}
  </>
);

/** @param {ShotProps} props */
const VortexShot = ({seconds, width, height, layout}) => {
  const {zoom, spin} = getVortexState(seconds);
  const cx = width / 2;
  const cy = height * 0.36;
  const eye = height * 0.34 * zoom;
  return (
    <AbsoluteFill
      style={{
        background: 'radial-gradient(ellipse at 50% 36%, #f4f9ff 0%, #b9d4f4 55%, #8db4ea 100%)',
      }}
    >
      <AbsoluteFill style={{filter: 'url(#beta-swirl)'}}>
        <div
          style={{
            background:
              'repeating-conic-gradient(from 0deg, #0b2262 0deg, #2a58b4 5deg, #102f78 9deg, #7aa6e8 12deg, #0b2262 16deg)',
            borderRadius: '50%',
            height: eye * 2,
            left: cx - eye,
            maskImage: 'radial-gradient(circle, black 0%, black 40%, rgba(0,0,0,0.6) 60%, transparent 76%)',
            position: 'absolute',
            top: cy - eye,
            transform: `rotate(${spin * 4}deg)`,
            width: eye * 2,
          }}
        />
      </AbsoluteFill>
      <Glint color="#cfe6ff" size={height * 0.16 * zoom} x={cx} y={cy} />
      <div
        style={{
          background: 'linear-gradient(90deg, transparent, #ffffff 50%, transparent)',
          height: 2,
          left: cx - eye * 1.1,
          position: 'absolute',
          top: cy - 1,
          width: eye * 2.2,
        }}
      />
      <AbsoluteFill style={{transform: `scale(${zoom}) rotate(${spin}deg)`, transformOrigin: `${cx}px ${cy}px`}}>
        <Img
          src={layout.ringImage}
          style={{
            height: ringReach * height * 2,
            left: cx - ringReach * height,
            position: 'absolute',
            top: cy - ringReach * height,
            width: ringReach * height * 2,
          }}
        />
      </AbsoluteFill>
    </AbsoluteFill>
  );
};

/** @param {ShotProps} props */
const BeamShot = ({seconds, width, height}) => {
  const t = progress(seconds, gachaBetaTimeline.vortexEnd, gachaBetaTimeline.beamEnd);
  return (
    <AbsoluteFill
      style={{
        background: 'linear-gradient(120deg, #1d4f9c 0%, #3d7fd0 55%, #7fb2ec 100%)',
      }}
    >
      {[
        {offset: 0, size: 0.34, alpha: 1},
        {offset: 0.18, size: 0.12, alpha: 0.7},
      ].map((beam, index) => (
        <div
          key={index}
          style={{
            background: `linear-gradient(90deg, transparent, ${withAlpha('#7ff4ff', 0.6 * beam.alpha)} 25%, #ffffff 50%, ${withAlpha('#7ff4ff', 0.6 * beam.alpha)} 75%, transparent)`,
            height: height * 2.2,
            left: width * (0.25 + beam.offset + 0.25 * t),
            position: 'absolute',
            top: -height * 0.6,
            transform: 'rotate(-28deg)',
            width: height * beam.size,
          }}
        />
      ))}
    </AbsoluteFill>
  );
};

/**
 * Low cloud banks over the horizon: flat sea on the left, a rising bank on the right.
 *
 * @param {{width: number, height: number, layout: BetaLayout, drift: number, drop?: number}} props
 *   `drift` and `drop` shift the bank left and down, in frame fractions.
 */
const CloudSea = ({width, height, layout, drift, drop = 0}) => (
  <>
    <div
      style={{
        background: 'linear-gradient(180deg, rgba(190,214,250,0) 0%, #b7cff3 30%, #e6f0ff 70%, #c4d8f6 100%)',
        height: height * 0.16,
        left: 0,
        position: 'absolute',
        top: height * (0.86 + drop),
        width,
      }}
    />
    <Img
      src={layout.bankImage}
      style={{
        height: height * cloudBandHeight,
        left: -drift * width,
        position: 'absolute',
        top: height * (cloudBandTop + drop),
        width: width * cloudBandWidth,
      }}
    />
  </>
);

/**
 * Turbulence displacement that twists the vortex eye's streaks into wisps.
 *
 * @param {{height: number, seed: number}} props
 */
const PaintFilters = ({height, seed}) => (
  <svg height={0} style={{position: 'absolute'}} width={0}>
    <defs>
      <filter height="140%" id="beta-swirl" width="140%" x="-20%" y="-20%">
        <feTurbulence baseFrequency={7 / height} numOctaves={1} seed={(seed + 7) % 1000} type="fractalNoise" />
        <feDisplacementMap in="SourceGraphic" scale={height * 0.12} xChannelSelector="R" yChannelSelector="G" />
      </filter>
    </defs>
  </svg>
);

/** @param {ShotProps} props */
const SkyShot = ({seconds, width, height, layout, palette}) => {
  const tl = gachaBetaTimeline;
  const head = getMeteorHead(seconds);
  const halo = getHaloState(seconds);
  const burst = getBurstState(seconds);
  const hx = head.x * width;
  const hy = head.y * height;
  const trailAngle = 180 + (Math.atan2(0.44 * height, 0.54 * width) * 180) / Math.PI;
  const sinceStop = Math.max(0, seconds - tl.meteorStop);
  const core = burst.tint > 0.3 ? '#ffe8f4' : '#ffffff';
  const headColor = halo.warmth > 0.5 ? '#ff8fb4' : palette.glow;
  return (
    <AbsoluteFill
      style={{
        background: 'linear-gradient(180deg, #141a52 0%, #23388a 45%, #4a72c8 78%, #8fb2ea 100%)',
      }}
    >
      {layout.stars.map((star, index) => (
        <div
          key={`star-${index}`}
          style={{
            background: '#ffffff',
            borderRadius: '50%',
            height: star.radius * height,
            left: star.x * width,
            opacity: 0.2 + 0.5 * getTwinkle(seconds, star),
            position: 'absolute',
            top: star.y * height * 0.6,
            width: star.radius * height,
          }}
        />
      ))}
      <AbsoluteFill
        style={{
          background: `radial-gradient(ellipse at ${head.x * 100}% ${head.y * 100}%, rgba(255,140,220,0.55) 0%, rgba(150,90,230,0.35) 35%, transparent 70%)`,
          opacity: burst.tint,
        }}
      />
      <div
        style={{
          left: hx,
          position: 'absolute',
          top: hy,
          transform: `rotate(${trailAngle}deg)`,
        }}
      >
        <div
          style={{
            background: `linear-gradient(90deg, ${withAlpha('#9fdcff', 0.6)}, ${withAlpha('#4f86f0', 0.4)} 30%, ${withAlpha('#3a5fd8', 0.15)} 70%, transparent)`,
            clipPath: 'polygon(0 35%, 100% 0, 100% 100%, 0 65%)',
            borderRadius: height,
            height: height * 0.3,
            position: 'absolute',
            top: -height * 0.15,
            width: width * 1.3,
          }}
        />
        <div
          style={{
            background: 'linear-gradient(90deg, #ffffff, #9ff4ff 20%, rgba(80,170,255,0.5) 55%, transparent)',
            clipPath: 'polygon(0 20%, 100% 45%, 100% 55%, 0 80%)',
            height: height * 0.024,
            position: 'absolute',
            top: -height * 0.012,
            width: width * 1.1,
          }}
        />
        {layout.slivers.map((sliver, index) => {
          const flow = (sliver.phase + seconds * (0.6 + head.speed)) % 1;
          return (
            <div
              key={`sliver-${index}`}
              style={{
                background: 'linear-gradient(90deg, #e9feff, #5fd8ff)',
                borderRadius: height,
                height: height * 0.008,
                left: (sliver.along * 0.5 + flow * 0.4) * width,
                opacity: (1 - flow) * (1 - burst.tint),
                position: 'absolute',
                top: sliver.side * height * (0.03 + flow * 0.12),
                width: sliver.length * width,
              }}
            />
          );
        })}
      </div>
      {sinceStop > 0
        ? layout.confetti.map((piece, index) => {
            const t = clamp01((sinceStop - piece.delay) / 1.2);
            if (t <= 0) {
              return null;
            }
            const radians = (piece.angle * Math.PI) / 180;
            const reach = piece.reach * height * (1 - (1 - t) ** 2);
            const size = piece.size * height;
            return (
              <div
                key={`confetti-${index}`}
                style={{
                  background: `radial-gradient(circle, #ffffff 0%, ${piece.color} 25%, ${withAlpha(piece.color, 0)} 70%)`,
                  borderRadius: '50%',
                  height: size * 3,
                  left: hx + Math.cos(radians) * reach - size,
                  opacity: 1 - t * 0.7,
                  position: 'absolute',
                  top: hy + Math.sin(radians) * reach - size,
                  width: size * 3,
                }}
              />
            );
          })
        : null}
      {halo.opacity > 0 ? (
        <div
          style={{
            background: `radial-gradient(${rainbow})`,
            borderRadius: '50%',
            height: halo.radius * height * 2,
            left: hx - halo.radius * height,
            maskImage:
              'conic-gradient(from 0deg, black 0deg 80deg, transparent 120deg 190deg, black 220deg 280deg, transparent 320deg 360deg)',
            opacity: halo.opacity,
            position: 'absolute',
            top: hy - halo.radius * height,
            width: halo.radius * height * 2,
          }}
        />
      ) : null}
      {burst.flare > 0 ? (
        <>
          <div
            style={{
              background:
                'linear-gradient(90deg, transparent, rgba(255,150,240,0.9) 30%, #ffffff 50%, rgba(255,150,240,0.9) 70%, transparent)',
              height: height * (0.006 + 0.01 * burst.flare),
              left: hx - width * 0.9 * burst.flare,
              position: 'absolute',
              top: hy - height * 0.008,
              width: width * 1.8 * burst.flare,
            }}
          />
          {Array.from({length: 16}, (_, index) => (
            <div
              key={`ray-${index}`}
              style={{
                background: 'linear-gradient(90deg, #ffffff, rgba(255,170,230,0.6) 40%, transparent)',
                height: 2,
                left: hx,
                opacity: burst.glow,
                position: 'absolute',
                top: hy - 1,
                transform: `rotate(${index * 22.5 + seconds * 30}deg)`,
                transformOrigin: '0 50%',
                width: height * (0.15 + (index % 3) * 0.1) * (0.5 + burst.glow),
              }}
            />
          ))}
        </>
      ) : null}
      <Glint
        color={headColor}
        core={core}
        rays={sinceStop > 0 ? 8 : 4}
        size={height * (0.1 + 0.12 * halo.opacity + 0.5 * burst.glow)}
        spin={sinceStop * 40}
        x={hx}
        y={hy}
      />
      <CloudSea
        drift={0.04 * progress(seconds, tl.beamEnd, tl.redFlashEnd)}
        height={height}
        layout={layout}
        width={width}
      />
    </AbsoluteFill>
  );
};

/** @param {ShotProps} props */
const VolleyShot = ({seconds, width, height, layout}) => {
  const red = getRedMeteor(seconds);
  const tilt = (Math.atan(0.12) * 180) / Math.PI;
  return (
    <AbsoluteFill
      style={{
        background: 'linear-gradient(180deg, #1c3a98 0%, #3466c8 50%, #6f9fe6 85%, #a9c8f2 100%)',
      }}
    >
      {volleyComets.map((comet, index) => {
        const head = getVolleyCometHead(comet, seconds);
        const size = comet.size * height;
        const x = head.x * width;
        const y = head.y * height;
        return (
          <div
            key={`comet-${index}`}
            style={{
              left: x,
              position: 'absolute',
              top: y,
              transform: `rotate(${-tilt}deg)`,
            }}
          >
            <div
              style={{
                background:
                  'linear-gradient(0deg, rgba(200,250,255,0.9), rgba(90,190,255,0.5) 25%, rgba(70,140,255,0.15) 60%, transparent)',
                borderRadius: size,
                clipPath: 'polygon(0 0, 100% 0, 75% 100%, 25% 100%)',
                height: height * 1.1,
                left: -size * 0.6,
                position: 'absolute',
                top: -height * 1.1 - size * 0.4,
                width: size * 1.2,
              }}
            />
            <div
              style={{
                background: 'radial-gradient(circle, rgba(127,230,255,0.7) 0%, transparent 70%)',
                borderRadius: '50%',
                height: size * 1.6,
                left: -size * 0.8,
                position: 'absolute',
                top: -size * 1.3,
                width: size * 1.6,
              }}
            />
            <div
              style={{
                background: 'linear-gradient(180deg, #ffffff, #b8f6ff 55%, #5fc8ff)',
                clipPath: 'polygon(50% 100%, 100% 45%, 50% 0%, 0% 45%)',
                height: size * 1.4,
                left: -size * 0.35,
                position: 'absolute',
                top: -size * 1.2,
                width: size * 0.7,
              }}
            />
          </div>
        );
      })}
      {red.flare > 0 ? (
        <>
          {red.falling > 0 ? (
            <div
              style={{
                background: 'linear-gradient(0deg, rgba(255,120,140,0.9), rgba(255,80,110,0.4) 35%, transparent)',
                borderRadius: height,
                height: height * 0.7 * red.falling + height * 0.05,
                left: red.x * width - height * 0.03,
                position: 'absolute',
                top: red.y * height - (height * 0.7 * red.falling + height * 0.05),
                transform: 'rotate(20deg)',
                transformOrigin: '50% 100%',
                width: height * 0.06,
              }}
            />
          ) : null}
          <div
            style={{
              background:
                'linear-gradient(90deg, transparent, rgba(255,120,130,0.9) 40%, #ffffff 50%, rgba(255,120,130,0.9) 60%, transparent)',
              height: 2 + 2 * red.flare,
              left: red.x * width - height * 0.5 * red.flare,
              position: 'absolute',
              top: red.y * height - 1,
              width: height * red.flare,
            }}
          />
          <Glint
            color="#ff4a64"
            core="#fff2f4"
            rays={6}
            size={height * 0.16 * red.flare}
            spin={seconds * 50}
            x={red.x * width}
            y={red.y * height}
          />
        </>
      ) : null}
      <CloudSea drift={0.1} drop={0.06} height={height} layout={layout} width={width} />
    </AbsoluteFill>
  );
};

/**
 * The rank emblem: a faceted rhombus crystal around the rank letters. Drawn
 * solid black for the silhouette and in the rarity colours on the card.
 *
 * @param {{size: number, silhouette: boolean, palette: import('./gacha-timeline.js').RarityPalette}} props
 */
const Emblem = ({size, silhouette, palette}) => {
  const fill = silhouette ? '#050305' : `linear-gradient(135deg, #ffffff 0%, ${palette.emblem} 100%)`;
  return (
    <div style={{height: size, position: 'relative', width: size}}>
      <div
        style={{
          background: fill,
          clipPath: 'polygon(50% 0%, 100% 50%, 50% 100%, 0% 50%)',
          inset: 0,
          position: 'absolute',
        }}
      />
      {silhouette ? null : (
        <div
          style={{
            background: `linear-gradient(135deg, ${withAlpha(palette.deep, 0.9)}, #0b0d2a)`,
            clipPath: 'polygon(50% 8%, 92% 50%, 50% 92%, 8% 50%)',
            inset: 0,
            position: 'absolute',
          }}
        />
      )}
      <div
        style={{
          alignItems: 'center',
          color: silhouette ? '#050305' : '#ffffff',
          display: 'flex',
          fontFamily: serifFont,
          fontSize: size * 0.3,
          fontStyle: 'italic',
          fontWeight: 700,
          inset: 0,
          justifyContent: 'center',
          letterSpacing: size * 0.01,
          position: 'absolute',
          textShadow: silhouette ? undefined : `0 0 ${size * 0.06}px ${palette.glow}`,
        }}
      >
        {getBetaRank()}
      </div>
    </div>
  );
};

/** @param {ShotProps} props */
const SilhouetteShot = ({seconds, width, height, layout, palette}) => {
  const tl = gachaBetaTimeline;
  const t = progress(seconds, tl.volleyEnd, tl.silhouetteEnd);
  const cx = width / 2;
  const cy = height / 2;
  const disc = height * (0.46 + 0.04 * t);
  const emblem = height * (0.62 + 0.06 * t);
  return (
    <AbsoluteFill
      style={{
        background: 'radial-gradient(ellipse, #5a4a50 0%, #2a2226 100%)',
      }}
    >
      <div
        style={{
          background: 'radial-gradient(circle, #ff8a96 0%, #e2384e 35%, #a51c32 75%, #5a0d1c 100%)',
          borderRadius: '50%',
          height: disc * 2,
          left: cx - disc,
          position: 'absolute',
          top: cy - disc,
          width: disc * 2,
        }}
      />
      <div
        style={{
          background: 'repeating-conic-gradient(from 0deg, rgba(255,210,220,0.55) 0deg 3deg, transparent 3deg 14deg)',
          borderRadius: '50%',
          height: disc * 1.6,
          left: cx - disc * 0.8,
          maskImage: 'radial-gradient(circle, transparent 18%, black 30%, transparent 70%)',
          position: 'absolute',
          top: cy - disc * 0.8,
          transform: `rotate(${seconds * 12}deg)`,
          width: disc * 1.6,
        }}
      />
      {[
        {w: 1.5, h: 0.75, turn: -20, thick: 0.06},
        {w: 1.2, h: 0.95, turn: 150, thick: 0.045},
        {w: 1.8, h: 0.6, turn: 200, thick: 0.035},
        {w: 1.0, h: 0.7, turn: 40, thick: 0.03},
      ].map((ribbon, index) => (
        <div
          key={`ribbon-${index}`}
          style={{
            borderRadius: '50%',
            borderTop: `${height * ribbon.thick}px solid #050305`,
            height: height * ribbon.h,
            left: cx - (height * ribbon.w) / 2,
            position: 'absolute',
            top: cy - (height * ribbon.h) / 2,
            transform: `rotate(${ribbon.turn + (index % 2 ? -1 : 1) * 25 * t}deg)`,
            width: height * ribbon.w,
          }}
        />
      ))}
      {layout.shards.map((shard, index) => {
        const offset = getShardOffset(shard, seconds);
        const size = shard.size * height;
        return (
          <div
            key={`shard-${index}`}
            style={{
              background: '#050305',
              height: size,
              left: cx + offset.dx * height - size / 2,
              opacity: offset.opacity,
              position: 'absolute',
              top: cy + offset.dy * height - size / 2,
              transform: `rotate(${offset.rotate}deg)`,
              width: size * (index % 3 === 0 ? 1.6 : 1),
            }}
          />
        );
      })}
      <div
        style={{
          left: cx - emblem / 2,
          position: 'absolute',
          top: cy - emblem / 2,
        }}
      >
        <Emblem palette={palette} silhouette size={emblem} />
      </div>
    </AbsoluteFill>
  );
};

/**
 * @param {{seconds: number, width: number, height: number, layout: BetaLayout, palette: import('./gacha-timeline.js').RarityPalette, label: string}} props
 */
const RevealShot = ({seconds, width, height, layout, palette, label}) => {
  const tl = gachaBetaTimeline;
  const settle = progress(seconds, tl.silhouetteEnd, gachaBetaTimeline.silhouetteEnd + 2.5);
  const plateIn = interpolate(seconds, [tl.plateIn, tl.plateIn + 0.4], [0, 1], {
    ...clamp,
    easing: Easing.out(Easing.cubic),
  });
  const emblem = height * 0.62;
  const labelLength = Array.from(label).length;
  const nameSize = height * (labelLength <= 10 ? 0.085 : labelLength <= 20 ? 0.065 : 0.05);
  const starSize = height * 0.042;
  return (
    <AbsoluteFill
      style={{
        background: `radial-gradient(ellipse at 60% 45%, ${withAlpha(palette.deep, 0.9)} 0%, #1a1c48 45%, #0b0c22 100%)`,
      }}
    >
      <AbsoluteFill style={{transform: `scale(${1.06 - 0.06 * settle})`}}>
        {layout.chains.map((chain, index) => (
          <div
            key={`chain-${index}`}
            style={{
              display: 'flex',
              gap: height * 0.012,
              left: (chain.x + chain.drift * settle) * width - width * 0.2,
              position: 'absolute',
              top: chain.y * height,
              transform: `rotate(${chain.angle}deg)`,
            }}
          >
            {Array.from({length: chain.count}, (_, link) => (
              <div
                key={link}
                style={{
                  background: withAlpha('#9fc4ff', 0.55 - link * 0.03),
                  border: `1px solid ${withAlpha('#dfeaff', 0.6)}`,
                  height: height * 0.035,
                  transform: 'rotate(45deg)',
                  width: height * 0.035,
                }}
              />
            ))}
          </div>
        ))}
        {layout.panels.map((panel, index) => (
          <div
            key={`panel-${index}`}
            style={{
              background: 'linear-gradient(135deg, #f0fbff, #8fdcff)',
              boxShadow: `0 0 ${height * 0.03}px #8fdcff`,
              height: panel.h * height,
              left: panel.x * width,
              opacity: 0.35 + 0.35 * Math.sin(seconds * 2 + panel.phase),
              position: 'absolute',
              top: panel.y * height,
              transform: `rotate(${panel.tilt}deg)`,
              width: panel.w * height,
            }}
          />
        ))}
        <div
          style={{
            background: `radial-gradient(circle, ${withAlpha(palette.glow, 0.5)} 0%, transparent 65%)`,
            borderRadius: '50%',
            height: emblem * 1.8,
            left: width * 0.6 - emblem * 0.9,
            position: 'absolute',
            top: height * 0.46 - emblem * 0.9,
            width: emblem * 1.8,
          }}
        />
        <div
          style={{
            left: width * 0.6 - emblem / 2,
            position: 'absolute',
            top: height * 0.46 - emblem / 2 + Math.sin(seconds * 1.5) * height * 0.008,
          }}
        >
          <Emblem palette={palette} silhouette={false} size={emblem} />
        </div>
      </AbsoluteFill>
      <div
        style={{
          left: width * 0.05,
          opacity: plateIn,
          position: 'absolute',
          top: height * 0.6,
          transform: `translateX(${(plateIn - 1) * width * 0.03}px)`,
        }}
      >
        <div style={{alignItems: 'center', display: 'flex', gap: height * 0.02}}>
          <div
            style={{
              background: `linear-gradient(135deg, #ffffff, ${palette.glow})`,
              boxShadow: `0 0 ${height * 0.02}px ${palette.glow}`,
              clipPath: 'polygon(50% 0%, 100% 50%, 50% 100%, 0% 50%)',
              flexShrink: 0,
              height: height * 0.1,
              width: height * 0.1,
            }}
          />
          <div style={{maxWidth: width * 0.42}}>
            <div
              style={{
                color: '#ffffff',
                fontSize: nameSize,
                fontWeight: 800,
                lineHeight: 1.1,
                overflowWrap: 'anywhere',
                textShadow: `0 ${height * 0.004}px ${height * 0.012}px rgba(0,0,0,0.7)`,
              }}
            >
              {label}
            </div>
            <div
              style={{
                color: '#ffffff',
                fontSize: height * 0.055,
                fontWeight: 700,
                lineHeight: 1.2,
                textShadow: `0 ${height * 0.004}px ${height * 0.012}px rgba(0,0,0,0.7)`,
              }}
            >
              {`Rank ${getBetaRank()}`}
            </div>
          </div>
        </div>
        <div
          style={{
            display: 'flex',
            gap: height * 0.006,
            marginLeft: height * 0.12,
            marginTop: height * 0.012,
          }}
        >
          {getStarRevealTimes().map((time, index) => {
            const pop = interpolate(seconds, [time, time + 0.12, time + 0.24], [0, 1.4, 1], clamp);
            return (
              <div
                key={index}
                style={{
                  height: starSize,
                  opacity: pop > 0 ? 1 : 0,
                  transform: `scale(${pop})`,
                  width: starSize,
                }}
              >
                <Star
                  fill={starColor}
                  innerRadius={starSize * 0.22}
                  outerRadius={starSize * 0.5}
                  points={5}
                  stroke="#fff4c2"
                  strokeWidth={1}
                />
              </div>
            );
          })}
        </div>
      </div>
    </AbsoluteFill>
  );
};

/**
 * @param {import('../schemas/gacha-request.js').GachaRenderRequest} props
 */
export const GachaBetaComposition = (props) => {
  const frame = useCurrentFrame();
  const {fps, width, height} = useVideoConfig();
  const seconds = frame / fps;
  const layout = useMemo(() => createBetaLayout(props.seed, width, height), [props.seed, width, height]);
  const palette = getRarityPalette(props.rarity);
  const shot = getShot(seconds);
  const flash = getFlash(seconds);
  const shared = {seconds, width, height, layout, palette};

  return (
    <AbsoluteFill
      style={{
        background: '#000000',
        fontFamily: sansFont,
        overflow: 'hidden',
      }}
    >
      <PaintFilters height={height} seed={props.seed} />
      {shot === 'vortex' ? <VortexShot {...shared} /> : null}
      {shot === 'beam' ? <BeamShot {...shared} /> : null}
      {shot === 'sky' ? <SkyShot {...shared} /> : null}
      {shot === 'volley' ? <VolleyShot {...shared} /> : null}
      {shot === 'silhouette' ? <SilhouetteShot {...shared} /> : null}
      {shot === 'reveal' ? <RevealShot {...shared} label={props.label} /> : null}
      {flash.opacity > 0 ? <AbsoluteFill style={{background: flash.color, opacity: flash.opacity}} /> : null}
      {flash.opacity > 0 && flash.color !== '#ffffff' && shot === 'sky' ? (
        <AbsoluteFill
          style={{
            background:
              'radial-gradient(circle at 58% 56%, #ffffff 0%, rgba(255,255,255,0.8) 14%, transparent 40%), repeating-conic-gradient(from 0deg at 58% 56%, rgba(255,235,240,0.85) 0deg 1.2deg, transparent 1.2deg 7deg)',
            opacity: flash.opacity,
          }}
        />
      ) : null}
    </AbsoluteFill>
  );
};
