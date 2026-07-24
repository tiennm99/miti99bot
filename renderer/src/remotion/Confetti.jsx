import {AbsoluteFill, interpolate, random, useCurrentFrame, useVideoConfig} from 'remotion';
import {createConfettiParticles, getConfettiParticleState} from './confetti-layout.js';

/**
 * Winner-celebration confetti burst. Renders nothing before `startFrame`, then
 * fires a deterministic paper-strip burst from the wheel center through the end
 * of the composition.
 *
 * @param {object} props
 * @param {number} props.startFrame frame the burst begins on (winner reveal)
 * @param {string[]} props.colors palette (theme slice colors)
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
  const progress = localFrame / lifetimeFrames;
  const opacity = interpolate(progress, [0, 0.06, 0.7, 1], [0, 1, 1, 0], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  });

  const particles = createConfettiParticles({
    colors,
    count,
    random: (key) => random(`${seed}-${key}`),
    size,
  });

  return (
    <AbsoluteFill style={{opacity, pointerEvents: 'none', zIndex: 10}}>
      {particles.map((particle, index) => {
        const state = getConfettiParticleState(particle, seconds);

        return (
          <div
            key={`confetti-${index}`}
            style={{
              background: particle.color,
              borderRadius: 1,
              height: particle.height,
              left: state.x,
              position: 'absolute',
              top: state.y,
              transform: `translate(-50%, -50%) rotate(${state.rotation}deg)`,
              transformOrigin: '50% 50%',
              width: particle.width,
            }}
          />
        );
      })}
    </AbsoluteFill>
  );
};
