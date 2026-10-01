import {useMemo} from 'react';
import {AbsoluteFill, Easing, interpolate, useCurrentFrame, useVideoConfig} from 'remotion';
import {
  gachaBetaTimeline,
  getBetaRank,
  getBurnGlint,
  getHeroMeteor,
  getSkyMeteorHead,
  meteorAngleDegrees,
  revealAnchor,
  skyMeteors,
} from './gacha-beta-scene.js';
import {createStarfield, getLabelFontSize, getRarityPalette, getTwinkle} from './gacha-timeline.js';

const sansFont =
  'Quicksand, Inter, "Noto Sans", "Noto Sans Vietnamese", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif';
const serifFont = '"Noto Serif", "Cormorant Garamond", Georgia, "Times New Roman", serif';

const clamp = /** @type {const} */ ({extrapolateLeft: 'clamp', extrapolateRight: 'clamp'});
const easeOut = /** @type {const} */ ({...clamp, easing: Easing.out(Easing.cubic)});

/**
 * @param {string} hex
 * @param {number} alpha
 */
const withAlpha = (hex, alpha) => {
  const value = Number.parseInt(hex.slice(1), 16);
  return `rgba(${(value >> 16) & 255}, ${(value >> 8) & 255}, ${value & 255}, ${alpha})`;
};

/** Narrows from full thickness at the head to a point at the end of the tail. */
const taper = 'polygon(0 0, 100% 46%, 100% 54%, 0 100%)';

/**
 * One meteor: a white-hot head inside a coloured halo, a bright tapered core
 * trail, and a wider, softer wisp around it. Drawn at the head and rotated so
 * the tail points back up the travel diagonal.
 *
 * @param {{x: number, y: number, length: number, thickness: number, color: string, core: string, opacity: number}} props
 */
const Meteor = ({x, y, length, thickness, color, core, opacity}) => (
  <div
    style={{
      left: x,
      opacity,
      position: 'absolute',
      top: y,
      transform: `rotate(${meteorAngleDegrees + 180}deg)`,
      transformOrigin: '0 0',
    }}
  >
    <div
      style={{
        background: `radial-gradient(circle, ${withAlpha(color, 0.35)} 0%, ${withAlpha(color, 0.12)} 40%, transparent 70%)`,
        borderRadius: '50%',
        height: thickness * 10,
        left: -thickness * 5,
        position: 'absolute',
        top: -thickness * 5,
        width: thickness * 10,
      }}
    />
    <div
      style={{
        background: `linear-gradient(90deg, ${withAlpha(color, 0.55)}, ${withAlpha(color, 0.18)} 45%, ${withAlpha(color, 0)})`,
        clipPath: taper,
        filter: `blur(${thickness * 0.5}px)`,
        height: thickness * 3.2,
        position: 'absolute',
        top: -thickness * 1.6,
        width: length * 1.1,
      }}
    />
    <div
      style={{
        background: `linear-gradient(90deg, ${core} 0%, ${color} 18%, ${withAlpha(color, 0.45)} 55%, ${withAlpha(color, 0)} 100%)`,
        clipPath: taper,
        height: thickness,
        position: 'absolute',
        top: -thickness / 2,
        width: length,
      }}
    />
    <div
      style={{
        background: core,
        borderRadius: '50%',
        boxShadow: `0 0 ${thickness * 1.5}px ${thickness * 0.5}px ${color}, 0 0 ${thickness * 4}px ${withAlpha(color, 0.6)}`,
        height: thickness * 1.15,
        left: -thickness * 0.6,
        position: 'absolute',
        top: -thickness * 0.575,
        width: thickness * 3,
      }}
    />
  </div>
);

/**
 * Hairline flourish beside the rank, ending in a dot, after the source
 * video's title card.
 *
 * @param {{height: number, side: -1 | 1, spread: number}} props
 */
const Swash = ({height, side, spread}) => (
  <div style={{alignItems: 'center', display: 'flex', flexDirection: side > 0 ? 'row' : 'row-reverse'}}>
    <div
      style={{
        background: `linear-gradient(${side > 0 ? 90 : 270}deg, #ffffff, rgba(255,255,255,0.15))`,
        height: 1.5,
        width: height * 0.22 * spread,
      }}
    />
    <div style={{background: '#ffffff', borderRadius: '50%', height: height * 0.012, width: height * 0.012}} />
  </div>
);

/**
 * A snow-rimmed ridge drawn as a huge ellipse, so its crest curves like the
 * source shot's wide lens.
 *
 * @param {{cx: number, cy: number, rx: number, ry: number, rotate: number, top: string, base: string}} props
 */
const Ridge = ({cx, cy, rx, ry, rotate, top, base}) => (
  <div
    style={{
      background: `linear-gradient(180deg, ${top} 0%, ${base} 14%, ${base} 100%)`,
      borderRadius: '50%',
      height: ry * 2,
      left: cx - rx,
      position: 'absolute',
      top: cy - ry,
      transform: `rotate(${rotate}deg)`,
      width: rx * 2,
    }}
  />
);

/**
 * @param {import('../schemas/gacha-request.js').GachaRenderRequest} props
 */
export const GachaBetaComposition = (props) => {
  const frame = useCurrentFrame();
  const {fps, width, height} = useVideoConfig();
  const seconds = frame / fps;
  const tl = gachaBetaTimeline;
  const palette = getRarityPalette(props.rarity);
  const stars = useMemo(() => createStarfield(110, props.seed), [props.seed]);

  const fadeIn = interpolate(seconds, [0, tl.fadeInEnd], [0, 1], clamp);
  const hero = getHeroMeteor(seconds, width, height);
  const glint = getBurnGlint(seconds);
  const labelIn = interpolate(seconds, [tl.labelIn, tl.labelIn + 0.6], [0, 1], easeOut);
  const rankIn = interpolate(seconds, [tl.rankIn, tl.rankIn + 0.5], [0, 1], easeOut);
  const anchor = {x: revealAnchor.x * width, y: revealAnchor.y * height};

  return (
    <AbsoluteFill style={{background: '#000000', fontFamily: sansFont, overflow: 'hidden'}}>
      <AbsoluteFill
        style={{
          background: 'linear-gradient(180deg, #040a26 0%, #0a1a46 42%, #1b3a6c 78%, #2f5684 100%)',
          opacity: fadeIn,
        }}
      >
        {stars.map((star, index) => {
          const size = star.radius * height * 0.8;
          return (
            <div
              key={`star-${index}`}
              style={{
                background: '#dfe8ff',
                borderRadius: '50%',
                height: size,
                left: star.x * width,
                opacity: 0.15 + 0.55 * getTwinkle(seconds, star),
                position: 'absolute',
                top: star.y * height * 0.95,
                width: size,
              }}
            />
          );
        })}

        {skyMeteors.map((meteor, index) => {
          const head = getSkyMeteorHead(meteor, seconds, width, height);
          return head ? (
            <Meteor
              color={meteor.color}
              core="#ffffff"
              key={`meteor-${index}`}
              length={meteor.length * height}
              opacity={1 - 0.7 * labelIn}
              thickness={meteor.width * height}
              x={head.x}
              y={head.y}
            />
          ) : null;
        })}

        {hero.visible > 0 ? (
          <Meteor
            color={palette.glow}
            core={palette.core}
            length={height * 0.75 * hero.tail}
            opacity={hero.visible}
            thickness={height * 0.024}
            x={hero.x}
            y={hero.y}
          />
        ) : null}

        <Ridge base="#0d1d3d" cx={width * 0.5} cy={height * 1.62} rotate={0} rx={width * 0.75} ry={height * 0.66} top="#1c3560" />
        <Ridge
          base="#1b3762"
          cx={-width * 0.04}
          cy={height * 1.1}
          rotate={-8}
          rx={width * 0.32}
          ry={height * 0.32}
          top="#8fb0d6"
        />
        <Ridge
          base="#1a3560"
          cx={width * 1.06}
          cy={height * 1.14}
          rotate={10}
          rx={width * 0.32}
          ry={height * 0.32}
          top="#88a9d0"
        />
        <AbsoluteFill style={{background: 'radial-gradient(ellipse at 50% 40%, transparent 55%, rgba(2,6,22,0.55) 100%)'}} />

        {glint > 0 ? (
          <>
            <div
              style={{
                background: `radial-gradient(circle, ${palette.core} 0%, ${withAlpha(palette.glow, 0.65)} 22%, transparent 65%)`,
                borderRadius: '50%',
                height: height * 0.5,
                left: anchor.x - height * 0.25,
                opacity: glint,
                position: 'absolute',
                top: anchor.y - height * 0.25,
                width: height * 0.5,
              }}
            />
            {[0, 90, 45, 135].map((rayAngle, index) => (
              <div
                key={`glint-${rayAngle}`}
                style={{
                  background: `linear-gradient(90deg, transparent, ${palette.core}, transparent)`,
                  height: 2,
                  left: anchor.x - height * 0.4,
                  opacity: glint * (index < 2 ? 1 : 0.5),
                  position: 'absolute',
                  top: anchor.y - 1,
                  transform: `rotate(${rayAngle}deg) scaleX(${index < 2 ? 1 : 0.5})`,
                  width: height * 0.8,
                }}
              />
            ))}
          </>
        ) : null}

        {labelIn > 0 ? (
          <div
            style={{
              alignItems: 'center',
              display: 'flex',
              flexDirection: 'column',
              left: 0,
              opacity: labelIn,
              position: 'absolute',
              right: 0,
              top: anchor.y - height * 0.18,
              transform: `translateY(${(1 - labelIn) * height * 0.03}px)`,
            }}
          >
            <div
              style={{
                alignItems: 'center',
                display: 'flex',
                gap: height * 0.02,
                opacity: rankIn,
                transform: `scale(${0.9 + 0.1 * rankIn})`,
              }}
            >
              <Swash height={height} side={-1} spread={rankIn} />
              <div
                style={{
                  color: '#ffffff',
                  fontFamily: serifFont,
                  fontSize: height * 0.15,
                  fontStyle: 'italic',
                  fontWeight: 300,
                  letterSpacing: height * 0.012,
                  lineHeight: 1,
                  textShadow: `0 0 ${height * 0.03}px ${withAlpha(palette.glow, 0.9)}`,
                }}
              >
                {getBetaRank()}
              </div>
              <Swash height={height} side={1} spread={rankIn} />
            </div>
            <div
              style={{
                color: '#ffffff',
                fontSize: getLabelFontSize(props.label, height),
                fontWeight: 800,
                lineHeight: 1.15,
                marginTop: height * 0.035,
                maxWidth: width * 0.8,
                overflowWrap: 'anywhere',
                textAlign: 'center',
                textShadow: `0 0 ${height * 0.025}px ${withAlpha(palette.glow, 0.85)}, 0 ${height * 0.006}px ${height * 0.02}px #040a26`,
              }}
            >
              {props.label}
            </div>
          </div>
        ) : null}
      </AbsoluteFill>
    </AbsoluteFill>
  );
};
