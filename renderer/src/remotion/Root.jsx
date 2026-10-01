import {Composition} from 'remotion';
import {GachaBetaComposition} from './GachaBetaComposition.jsx';
import {GachaComposition} from './GachaComposition.jsx';
import {gachaBetaTotalSeconds} from './gacha-beta-scene.js';
import {gachaTotalSeconds} from './gacha-timeline.js';
import {WheelComposition} from './WheelComposition.jsx';
import {gachaFrameSizes} from '../schemas/gacha-request.js';

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
const defaultGachaProps = {
  label: 'Pizza',
  rarity: 5,
  fps: 24,
  width: 640,
  seed: 7,
};

/**
 * @param {{fps: number, width: number}} props
 * @param {number} [totalSeconds]
 */
export const getGachaCompositionMetadata = (props, totalSeconds = gachaTotalSeconds) => {
  const fps = props.fps || defaultGachaProps.fps;
  const width = props.width in gachaFrameSizes ? props.width : defaultGachaProps.width;
  return {
    durationInFrames: Math.ceil(totalSeconds * fps),
    fps,
    height: gachaFrameSizes[/** @type {640 | 854} */ (width)],
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
      calculateMetadata={({props}) => getGachaCompositionMetadata(props)}
      component={GachaComposition}
      defaultProps={defaultGachaProps}
      durationInFrames={Math.ceil(gachaTotalSeconds * defaultGachaProps.fps)}
      fps={defaultGachaProps.fps}
      height={gachaFrameSizes[defaultGachaProps.width]}
      id="GachaWish"
      width={defaultGachaProps.width}
    />
    <Composition
      calculateMetadata={({props}) => getGachaCompositionMetadata(props, gachaBetaTotalSeconds)}
      component={GachaBetaComposition}
      defaultProps={defaultGachaProps}
      durationInFrames={Math.ceil(gachaBetaTotalSeconds * defaultGachaProps.fps)}
      fps={defaultGachaProps.fps}
      height={gachaFrameSizes[defaultGachaProps.width]}
      id="GachaBetaWish"
      width={defaultGachaProps.width}
    />
  </>
);
