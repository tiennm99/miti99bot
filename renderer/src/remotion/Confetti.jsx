import {AbsoluteFill, interpolate, random, useCurrentFrame, useVideoConfig} from 'remotion';
import {createConfettiParticles, getConfettiParticleState} from './confetti-layout.js';

/**
 * Winner-celebration confetti burst. Renders nothing before `startFrame`, then
 * fires a deterministic two-cannon burst through the end of the composition.
 *
 * @param {object} props
 * @param {number} props.startFrame frame the burst begins on (winner reveal)
 * @param {string[]} props.colors palette (theme confetti colors)
 * @param {number|string} props.seed reproducibility seed (e.g. winner index)
 * @param {number} [props.count] particle count
 */
export const Confetti = ({startFrame, colors, seed, count = 70}) => {
  const frame = useCurrentFrame();
  const {durationInFrames, fps, height, width} = useVideoConfig();

  if (frame < startFrame) {
    return null;
  }

  const size = Math.min(width, height);
  const lifetimeFrames = Math.max(1, durationInFrames - 1 - startFrame);
  const localFrame = frame - startFrame;
  const seconds = localFrame / fps;
  // No fade-in: the opening frame is the only one where particles are still
  // clustered at the muzzles, and fading it out threw the pop away.
  const fadeFrames = Math.min(4, Math.max(1, Math.round(lifetimeFrames * 0.25)));
  const fade = interpolate(localFrame, [lifetimeFrames - fadeFrames, lifetimeFrames], [1, 0], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  });
  // Quantized so the tail introduces at most a handful of blended colors: a
  // 256-entry GIF palette cannot afford a fresh alpha level on every frame.
  const opacity = Math.ceil(fade * 4) / 4;

  const particles = createConfettiParticles({
    colors,
    count,
    random: (key) => random(`${seed}-${key}`),
    size,
    windowSeconds: lifetimeFrames / fps,
  });

  return (
    // Above the winner pill: the pill covers the hub the burst arcs across, and
    // behind it the celebration lost most of its particles at exactly the moment
    // it fires.
    <AbsoluteFill style={{opacity, pointerEvents: 'none', zIndex: 20}}>
      {particles.map((particle, index) => {
        // Staggered launch: an unlaunched particle would otherwise stack on its
        // muzzle and the pile reads as a clump of paper sitting in the corner.
        if (seconds < particle.delay) {
          return null;
        }

        const state = getConfettiParticleState(particle, seconds);

        return (
          <div
            key={`confetti-${index}`}
            style={{
              background: particle.color,
              // Hard 1px ring, no blur: separates chips from same-colored
              // slices without adding a palette entry, since white already
              // strokes the slice borders.
              boxShadow: '0 0 0 1px #ffffff',
              height: particle.height,
              left: state.x,
              position: 'absolute',
              top: state.y,
              // scaleX runs along the body's own long axis, and the rotation is
              // already aligned with travel while streaking, so this elongates
              // in the direction of motion and leaves the short axis at its
              // quantisation-safe minimum.
              transform: `translate(-50%, -50%) rotate(${state.rotation}deg) scaleX(${state.stretch})`,
              transformOrigin: '50% 50%',
              width: particle.width,
            }}
          />
        );
      })}
    </AbsoluteFill>
  );
};
