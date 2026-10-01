import {Composition} from 'remotion';
import {GenshinComposition} from './GenshinComposition.jsx';
import {genshinFrameSizes, genshinTotalSeconds} from './genshin-timeline.js';
import {WheelComposition} from './WheelComposition.jsx';

/** @type {import('../schemas/wheel-request.js').WheelRenderRequest} */
const defaultProps = {
  options: ['alice', 'bob', 'carol', 'dave', 'erin', 'frank', 'grace', 'heidi'],
  winnerIndex: 2,
  durationMs: 6500,
  holdMs: 1200,
  fps: 15,
  size: 512,
  theme: 'classic',
};

/**
 * @param {{durationMs: number, holdMs: number, fps: number, size: number}} props
 */
export const getCompositionMetadata = (props) => {
  const fps = props.fps || defaultProps.fps;
  const totalMs = (props.durationMs || defaultProps.durationMs) + (props.holdMs || defaultProps.holdMs);
  const durationInFrames = Math.max(1, Math.ceil((totalMs / 1000) * fps));
  const size = props.size || defaultProps.size;

  return {
    durationInFrames,
    fps,
    height: size,
    width: size,
  };
};

/** @type {import('../schemas/gacha-request.js').GachaRenderRequest} */
const defaultGenshinProps = {
  label: 'Pizza',
  rarity: 5,
  fps: 24,
  width: 640,
  seed: 7,
};

/**
 * @param {{fps: number, width: number}} props
 */
export const getGenshinCompositionMetadata = (props) => {
  const fps = props.fps || defaultGenshinProps.fps;
  const width = props.width in genshinFrameSizes ? props.width : defaultGenshinProps.width;
  return {
    durationInFrames: Math.ceil(genshinTotalSeconds * fps),
    fps,
    height: genshinFrameSizes[/** @type {640 | 854} */ (width)],
    width,
  };
};

export const RemotionRoot = () => (
  <>
    <Composition
      calculateMetadata={({props}) => getCompositionMetadata(props)}
      component={WheelComposition}
      defaultProps={defaultProps}
      durationInFrames={Math.ceil(((defaultProps.durationMs + defaultProps.holdMs) / 1000) * defaultProps.fps)}
      fps={defaultProps.fps}
      height={defaultProps.size}
      id="WheelGif"
      width={defaultProps.size}
    />
    <Composition
      calculateMetadata={({props}) => getGenshinCompositionMetadata(props)}
      component={GenshinComposition}
      defaultProps={defaultGenshinProps}
      durationInFrames={Math.ceil(genshinTotalSeconds * defaultGenshinProps.fps)}
      fps={defaultGenshinProps.fps}
      height={genshinFrameSizes[defaultGenshinProps.width]}
      id="GenshinWish"
      width={defaultGenshinProps.width}
    />
  </>
);
