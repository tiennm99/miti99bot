import {useMemo} from 'react';
import {Star} from '@remotion/shapes';
import {AbsoluteFill, Easing, interpolate, useCurrentFrame, useVideoConfig} from 'remotion';
import {
  gachaBetaTimeline,
  getBetaRank,
  getBetaShake,
  getCamera,
  getCometGlow,
  getCometPosition,
  heroCloud,
  pierceTime,
  project,
} from './gacha-beta-scene.js';
import {createRandom, createStarfield, getLabelFontSize, getRarityPalette, getTwinkle} from './gacha-timeline.js';

const baseFont =
  'Quicksand, Inter, "Noto Sans", "Noto Sans Vietnamese", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif';

const clamp = /** @type {const} */ ({extrapolateLeft: 'clamp', extrapolateRight: 'clamp'});

/** A luminous night sky behind flat, outline-free cloud shading. */
const skyBackground =
  'radial-gradient(ellipse at 50% 105%, #7886ce 0%, #353f83 32%, #171b49 65%, #090e29 100%)';
const cloudShade = 'radial-gradient(circle at 38% 32%, #f6f4ff 0% 44%, #cdd1f6 44% 70%, #959cdb 70%)';
const heroShade = 'radial-gradient(circle at 38% 30%, #ffffff 0% 46%, #d8dcfb 46% 72%, #a0a8e4 72%)';

const trailSamples = 44;
const trailStep = 0.01;

/**
 * Puffs that make one toon cloud, in units of the cloud radius.
 *
 * @param {() => number} random
 * @param {number} count
 */
const createPuffs = (random, count) =>
  Array.from({length: count}, (_, index) => {
    const angle = (index / count) * Math.PI * 2 + random() * 0.6;
    const reach = index === 0 ? 0 : 0.35 + random() * 0.45;
    return {
      dx: Math.cos(angle) * reach,
      dy: Math.sin(angle) * reach * 0.45 + (random() - 0.3) * 0.15,
      r: 0.38 + random() * 0.3,
    };
  });

/**
 * Every seeded layout for one roll: stars, constellation links, and the
 * clouds scattered through the sky.
 *
 * @param {number} seed
 */
const createBetaScene = (seed) => {
  const random = createRandom(seed + 101);
  const stars = createStarfield(80, seed);
  const links = Array.from({length: 7}, () => {
    const a = Math.floor(random() * 24);
    return [a, a + 1 + Math.floor(random() * 3)];
  });
  const clouds = Array.from({length: 14}, () => ({
    x: (random() - 0.5) * 3200,
    y: 80 + random() * 420,
    z: 1100 + random() * 5200,
    radius: 160 + random() * 220,
    puffs: createPuffs(random, 6),
  })).sort((a, b) => b.z - a.z);
  const heroPuffs = createPuffs(random, 13);
  const confetti = Array.from({length: 34}, () => ({
    x: random(),
    offset: random(),
    speed: 0.18 + random() * 0.3,
    size: 0.02 + random() * 0.025,
    spin: (random() - 0.5) * 400,
    hue: Math.floor(random() * 4),
  }));
  return {stars, links, clouds, heroPuffs, confetti};
};

const confettiColors = ['#ffd36b', '#ff8cd8', '#8cff9a', '#8fd0ff'];

/**
 * @param {string} hex
 * @param {number} alpha
 */
const withAlpha = (hex, alpha) => {
  const value = Number.parseInt(hex.slice(1), 16);
  return `rgba(${(value >> 16) & 255}, ${(value >> 8) & 255}, ${value & 255}, ${alpha})`;
};

/**
 * @param {{size: number, fill: string, shade: string}} props
 */
const ToonStar = ({size, fill, shade}) => (
  <div style={{height: size, position: 'relative', width: size}}>
    <Star
      cornerRadius={size * 0.035}
      fill={shade}
      innerRadius={size * 0.23}
      outerRadius={size / 2}
      points={5}
      style={{height: size, position: 'absolute', width: size}}
    />
    <div style={{clipPath: 'polygon(0 0, 100% 0, 100% 42%, 50% 62%, 0 42%)', inset: 0, position: 'absolute'}}>
      <Star
        cornerRadius={size * 0.035}
        fill={fill}
        innerRadius={size * 0.23}
        outerRadius={size / 2}
        points={5}
        style={{height: size, width: size}}
      />
    </div>
  </div>
);

/**
 * @typedef {import('../schemas/gacha-request.js').GachaRenderRequest} GachaRenderRequest
 */

/**
 * Beta wish: a cosmic, astrology-themed night sky in toon shading. The
 * camera dollies toward a hero cloud, a comet bursts through it, the camera
 * chases the comet as it flares, and after the burst the result appears with
 * its rank.
 *
 * @param {GachaRenderRequest} props
 */
export const GachaBetaComposition = (props) => {
  const frame = useCurrentFrame();
  const {fps, width, height} = useVideoConfig();
  const seconds = frame / fps;
  const tl = gachaBetaTimeline;
  const palette = getRarityPalette(props.rarity);
  const scene = useMemo(() => createBetaScene(props.seed), [props.seed]);

  const camera = getCamera(seconds);
  const shake = getBetaShake(seconds, height);
  const comet = getCometPosition(seconds);
  const cometScreen = project(comet, camera, width, height);
  const glow = getCometGlow(seconds);
  const cometVisible = seconds >= tl.cometAppear && seconds < tl.flashPeak;
  const burst = interpolate(seconds, [tl.flightEnd - 0.1, tl.flashPeak], [0, 1], clamp);
  const flash = interpolate(seconds, [tl.flightEnd, tl.flashPeak, tl.flashEnd], [0, 1, 0], clamp);
  const cardVisible = seconds >= tl.cardIn;
  const skyIn = interpolate(seconds, [0, 0.65], [0, 1], clamp);
  const revealIn = interpolate(seconds, [tl.cardIn, tl.flashEnd + 0.2], [0, 1], clamp);
  const pierceBurst = interpolate(seconds, [pierceTime, pierceTime + 0.9], [0, 1], {
    ...clamp,
    easing: Easing.out(Easing.cubic),
  });
  const speedLines = interpolate(
    seconds,
    [tl.pierceEnd, tl.pierceEnd + 0.4, tl.flightEnd - 0.6, tl.flightEnd],
    [0, 1, 1, 0],
    clamp,
  );

  /** @type {{depth: number, node: import('react').ReactNode}[]} */
  const drawables = [];

  for (const [index, cloud] of scene.clouds.entries()) {
    const screen = project(cloud, camera, width, height);
    if (!screen) {
      continue;
    }
    const radius = cloud.radius * screen.scale;
    drawables.push({
      depth: screen.depth,
      node: (
        <div
          key={`cloud-${index}`}
          style={{left: screen.x, opacity: Math.min(1, 900 / screen.depth + 0.25), position: 'absolute', top: screen.y}}
        >
          {cloud.puffs.map((puff, puffIndex) => (
            <div
              key={puffIndex}
              style={{
                background: cloudShade,
                borderRadius: '50%',
                height: puff.r * radius * 2,
                left: puff.dx * radius - puff.r * radius,
                position: 'absolute',
                top: puff.dy * radius - puff.r * radius,
                width: puff.r * radius * 2,
              }}
            />
          ))}
        </div>
      ),
    });
  }

  const hero = project(heroCloud, camera, width, height);
  if (hero) {
    const radius = heroCloud.radius * hero.scale;
    drawables.push({
      depth: hero.depth,
      node: (
        <div key="hero" style={{left: hero.x, opacity: skyIn, position: 'absolute', top: hero.y}}>
          {scene.heroPuffs.map((puff, index) => {
            const distance = Math.hypot(puff.dx, puff.dy) || 1;
            const push = pierceBurst * (0.6 + (1 - Math.min(1, distance)) * 1.4);
            const angle = (index / scene.heroPuffs.length) * Math.PI * 2;
            const ox = Math.cos(angle) * push;
            const oy = Math.sin(angle) * push;
            const opacity = 1 - pierceBurst * (distance < 0.5 ? 1 : 0.65);
            return (
              <div
                key={index}
                style={{
                  background: heroShade,
                  borderRadius: '50%',
                  height: puff.r * radius * 2,
                  left: (puff.dx + ox) * radius - puff.r * radius,
                  opacity,
                  position: 'absolute',
                  top: (puff.dy + oy) * radius - puff.r * radius,
                  transform: `rotate(${puff.dx * 12}deg) scale(${1 + pierceBurst * 0.3}, 0.78)`,
                  width: puff.r * radius * 2,
                }}
              />
            );
          })}
        </div>
      ),
    });
  }

  if (cometVisible && cometScreen) {
    const trail = [];
    for (let index = trailSamples - 1; index >= 1; index -= 1) {
      const past = seconds - index * trailStep;
      if (past < tl.cometAppear) {
        continue;
      }
      const point = project(getCometPosition(past), camera, width, height);
      if (!point) {
        continue;
      }
      const fade = 1 - index / trailSamples;
      const radius = (14 + 26 * glow) * point.scale * fade;
      trail.push(
        <div
          key={`trail-${index}`}
          style={{
            background: `radial-gradient(circle, ${palette.core} 0% 30%, ${palette.glow} 30% 62%, ${withAlpha(palette.glow, 0.35)} 62% 100%)`,
            borderRadius: '50%',
            height: radius * 2,
            left: point.x - radius,
            opacity: fade,
            position: 'absolute',
            top: point.y - radius,
            width: radius * 2,
          }}
        />,
      );
    }
    const headSize = (34 + 60 * glow + 90 * burst) * cometScreen.scale;
    const flare = headSize * (1.4 + glow * 3);
    drawables.push({
      depth: cometScreen.depth,
      node: (
        <div key="comet" style={{inset: 0, position: 'absolute'}}>
          {trail}
          <div
            style={{
              background: `radial-gradient(circle, ${withAlpha(palette.glow, 0.7)} 0% 35%, ${withAlpha(palette.glow, 0.3)} 35% 60%, transparent 60%)`,
              borderRadius: '50%',
              height: headSize * 3.2,
              left: cometScreen.x - headSize * 1.6,
              position: 'absolute',
              top: cometScreen.y - headSize * 1.6,
              width: headSize * 3.2,
            }}
          />
          {[0, 90, 45, 135].map((angle, index) => (
            <div
              key={`flare-${angle}`}
              style={{
                background: `linear-gradient(90deg, transparent 0%, ${palette.core} 50%, transparent 100%)`,
                clipPath: 'polygon(0% 50%, 50% 0%, 100% 50%, 50% 100%)',
                height: Math.max(3, headSize * (index < 2 ? 0.16 : 0.09)),
                left: cometScreen.x - (flare * (index < 2 ? 1 : 0.6)) / 2,
                opacity: index < 2 ? 1 : glow,
                position: 'absolute',
                top: cometScreen.y - Math.max(1.5, headSize * (index < 2 ? 0.08 : 0.045)),
                transform: `rotate(${angle + seconds * 20}deg)`,
                width: flare * (index < 2 ? 1 : 0.6),
              }}
            />
          ))}
          <div
            style={{
              left: cometScreen.x - headSize / 2,
              position: 'absolute',
              top: cometScreen.y - headSize / 2,
              transform: `perspective(${height}px) rotateY(${Math.sin(seconds * 4) * 32}deg) rotateZ(${seconds * 100}deg)`,
            }}
          >
            <ToonStar fill="#ffffff" shade={palette.core} size={headSize} />
          </div>
          {burst > 0 ? (
            <div
              style={{
                background: `repeating-conic-gradient(from ${seconds * 60}deg, #ffffff 0deg 9deg, ${palette.glow} 9deg 22.5deg)`,
                clipPath:
                  'polygon(50% 0%, 58% 34%, 85% 15%, 66% 42%, 100% 50%, 66% 58%, 85% 85%, 58% 66%, 50% 100%, 42% 66%, 15% 85%, 34% 58%, 0% 50%, 34% 42%, 15% 15%, 42% 34%)',
                height: height * 2.2 * burst,
                left: cometScreen.x - height * 1.1 * burst,
                position: 'absolute',
                top: cometScreen.y - height * 1.1 * burst,
                width: height * 2.2 * burst,
              }}
            />
          ) : null}
        </div>
      ),
    });
  }

  // While the comet hides behind the hero cloud, its light spills around the
  // cloud's rim so it visibly appears from behind before it breaks through.
  const backlight = interpolate(seconds, [tl.cometAppear, tl.pierceStart, pierceTime], [0, 1, 0.6], clamp);
  if (backlight > 0 && seconds < pierceTime) {
    const light = project({...comet, y: heroCloud.y}, camera, width, height);
    if (light) {
      const radius = heroCloud.radius * 1.7 * light.scale;
      drawables.push({
        depth: light.depth + 1,
        node: (
          <div
            key="backlight"
            style={{
              background: `radial-gradient(circle, ${withAlpha(palette.core, 0.95)} 0% 40%, ${withAlpha(palette.glow, 0.75)} 40% 62%, ${withAlpha(palette.glow, 0.25)} 62% 80%, transparent 80%)`,
              borderRadius: '50%',
              height: radius * 2,
              left: light.x - radius,
              opacity: backlight,
              position: 'absolute',
              top: light.y - radius,
              width: radius * 2,
            }}
          />
        ),
      });
    }
  }

  if (hero && seconds >= pierceTime && seconds < pierceTime + 0.65) {
    const impact = (seconds - pierceTime) / 0.65;
    const diameter = heroCloud.radius * hero.scale * (0.4 + impact * 3.5);
    drawables.push({
      depth: hero.depth - 1,
      node: (
        <div
          key="cloud-impact"
          style={{
            border: `${Math.max(1, 8 * (1 - impact))}px solid ${palette.core}`,
            borderRadius: '50%',
            boxShadow: `0 0 ${height * 0.04}px ${palette.glow}`,
            height: diameter,
            left: hero.x - diameter / 2,
            opacity: 1 - impact,
            position: 'absolute',
            top: hero.y - diameter / 2,
            transform: 'rotateX(48deg)',
            width: diameter,
          }}
        />
      ),
    });
  }

  drawables.sort((a, b) => b.depth - a.depth);

  const rank = getBetaRank();
  const cardPop = interpolate(seconds, [tl.cardIn, tl.cardIn + 0.35, tl.cardIn + 0.55], [0.6, 1.08, 1], {
    ...clamp,
    easing: Easing.out(Easing.cubic),
  });
  const rankPop = interpolate(seconds, [tl.rankIn, tl.rankIn + 0.18, tl.rankIn + 0.36], [0, 1.35, 1], clamp);

  return (
    <AbsoluteFill style={{background: '#0a0c2c', fontFamily: baseFont, overflow: 'hidden'}}>
      <AbsoluteFill
        style={{
          opacity: skyIn,
          transform: `translate(${shake.x}px, ${shake.y}px) rotate(${camera.roll}deg) scale(1.18)`,
        }}
      >
        <AbsoluteFill style={{background: skyBackground}} />

        <svg
          height={height}
          style={{left: 0, opacity: 0.32, position: 'absolute', top: 0}}
          viewBox={`0 0 ${width} ${height}`}
          width={width}
        >
          <g transform={`translate(${width / 2 - camera.x * 0.03} ${height * 0.4}) rotate(${seconds * 6})`}>
            <circle fill="none" r={height * 0.46} stroke="#aab4ff" strokeWidth={1.5} />
            <circle fill="none" r={height * 0.38} stroke="#aab4ff" strokeDasharray="2 6" strokeWidth={1} />
            {Array.from({length: 12}, (_, index) => {
              const angle = (index / 12) * Math.PI * 2;
              return (
                <g key={`tick-${index}`}>
                  <line
                    stroke="#aab4ff"
                    strokeWidth={1.5}
                    x1={Math.cos(angle) * height * 0.38}
                    x2={Math.cos(angle) * height * 0.46}
                    y1={Math.sin(angle) * height * 0.38}
                    y2={Math.sin(angle) * height * 0.46}
                  />
                  <circle
                    cx={Math.cos(angle + 0.26) * height * 0.42}
                    cy={Math.sin(angle + 0.26) * height * 0.42}
                    fill="#d8dcff"
                    r={2.2}
                  />
                </g>
              );
            })}
          </g>
          {scene.links.map(([a, b], index) => {
            const from = scene.stars[a ?? 0];
            const to = scene.stars[b ?? 0];
            if (!from || !to) {
              return null;
            }
            return (
              <line
                key={`link-${index}`}
                stroke="#c8d0ff"
                strokeWidth={1}
                x1={from.x * width}
                x2={to.x * width}
                y1={from.y * height * 0.7}
                y2={to.y * height * 0.7}
              />
            );
          })}
        </svg>

        {scene.stars.map((star, index) => {
          const twinkle = getTwinkle(seconds, star);
          const size = star.radius * 2 * height * (0.8 + 0.7 * twinkle);
          return (
            <div
              key={`star-${index}`}
              style={{
                background: '#ffffff',
                borderRadius: '50%',
                boxShadow: twinkle > 0.4 ? `0 0 ${size * 2}px rgba(200, 220, 255, ${twinkle})` : undefined,
                height: size,
                left: star.x * width - camera.x * 0.02,
                opacity: 0.25 + 0.75 * twinkle,
                position: 'absolute',
                top: star.y * height * 0.7,
                width: size,
              }}
            />
          );
        })}

        {drawables.map((drawable) => drawable.node)}

        {speedLines > 0 && cometScreen
          ? Array.from({length: 18}, (_, index) => {
              const angle = (index / 18) * 360 + ((index * 37) % 11);
              const reach = ((seconds * 2.2 + index * 0.137) % 1) * width * 0.7;
              return (
                <div
                  key={`speed-${index}`}
                  style={{
                    background: 'linear-gradient(90deg, transparent, rgba(255,255,255,0.75))',
                    height: 2,
                    left: cometScreen.x,
                    opacity: speedLines * 0.7,
                    position: 'absolute',
                    top: cometScreen.y,
                    transform: `rotate(${angle}deg) translateX(${height * 0.25 + reach}px)`,
                    transformOrigin: '0 50%',
                    width: width * 0.18,
                  }}
                />
              );
            })
          : null}
      </AbsoluteFill>

      {cardVisible ? (
        <AbsoluteFill
          style={{background: `radial-gradient(ellipse, ${withAlpha(palette.deep, 0.9)}, #0a0c2cee)`, opacity: revealIn}}
        >
          <div
            style={{
              background: `repeating-conic-gradient(from ${seconds * 4}deg, ${withAlpha(palette.glow, 0.16)} 0deg 5deg, transparent 5deg 30deg)`,
              borderRadius: '50%',
              height: width * 1.5,
              left: width / 2 - width * 0.75,
              position: 'absolute',
              top: height * 0.42 - width * 0.75,
              width: width * 1.5,
            }}
          />
          {scene.confetti.map((piece, index) => {
            const y = ((piece.offset + (seconds - tl.cardIn) * piece.speed) % 1.2) - 0.1;
            const size = piece.size * height;
            return (
              <div
                key={`confetti-${index}`}
                style={{
                  left: piece.x * width,
                  position: 'absolute',
                  top: y * height,
                  transform: `rotate(${seconds * piece.spin}deg)`,
                }}
              >
                <ToonStar fill="#ffffff" shade={confettiColors[piece.hue] ?? '#ffd36b'} size={size} />
              </div>
            );
          })}
          <div
            style={{
              border: `1px solid ${withAlpha(palette.core, 0.35)}`,
              borderRadius: '50%',
              height: height * 0.8,
              left: width / 2 - height * 0.4,
              position: 'absolute',
              top: height * 0.5 - height * 0.4,
              transform: `rotate(${seconds * 5}deg)`,
              width: height * 0.8,
            }}
          >
            {Array.from({length: 12}, (_, index) => (
              <div
                key={index}
                style={{
                  background: palette.core,
                  height: height * 0.018,
                  left: '50%',
                  opacity: 0.65,
                  position: 'absolute',
                  top: '50%',
                  transform: `rotate(${index * 30}deg) translateY(${-height * 0.4}px)`,
                  width: 2,
                }}
              />
            ))}
          </div>
          <div
            style={{
              alignItems: 'center',
              display: 'flex',
              flexDirection: 'column',
              inset: 0,
              justifyContent: 'center',
              position: 'absolute',
              transform: `scale(${cardPop})`,
            }}
          >
            <div
              style={{
                background: 'linear-gradient(180deg, #fffbe6 0% 40%, #ffd36b 40% 70%, #f0a92c 70%)',
                color: 'transparent',
                fontSize: height * 0.3,
                fontStyle: 'italic',
                fontWeight: 900,
                letterSpacing: height * 0.005,
                lineHeight: 1,
                opacity: rankPop > 0 ? 1 : 0,
                paddingRight: height * 0.03,
                transform: `scale(${rankPop})`,
                WebkitBackgroundClip: 'text',
                backgroundClip: 'text',
                filter: `drop-shadow(0 0 ${height * 0.04}px ${palette.glow})`,
              }}
            >
              {rank}
            </div>
            <div
              style={{
                color: '#ffffff',
                fontSize: getLabelFontSize(props.label, height) * 1.1,
                fontWeight: 900,
                lineHeight: 1.15,
                marginTop: height * 0.03,
                maxWidth: width * 0.8,
                overflowWrap: 'anywhere',
                textAlign: 'center',
                textShadow: `0 0 ${height * 0.03}px ${withAlpha(palette.glow, 0.8)}`,
              }}
            >
              {props.label}
            </div>
          </div>
        </AbsoluteFill>
      ) : null}

      {flash > 0 ? <AbsoluteFill style={{background: '#ffffff', opacity: flash}} /> : null}
    </AbsoluteFill>
  );
};
