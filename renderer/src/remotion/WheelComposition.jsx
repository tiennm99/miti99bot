import React from 'react';
import {AbsoluteFill, Easing, interpolate, useCurrentFrame, useVideoConfig} from 'remotion';
import {Circle, Pie} from '@remotion/shapes';
import {
  cssDegreesToPieRadians,
  getFinalWheelRotationDegrees,
  getSliceCenterDegrees,
  getSliceDegrees,
  getWheelRotationDegrees,
  rightPointerClipPath,
} from './wheel-layout.js';
import {getRadialLabelLayout} from './wheel-label-layout.js';
import {getTheme} from './themes.js';
import {Confetti} from './Confetti.jsx';

const baseFont =
  'Quicksand, Inter, "Noto Sans", "Noto Sans Vietnamese", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif';

/**
 * @typedef {import('../schemas/wheel-request.js').WheelRenderRequest} WheelRenderRequest
 */

/**
 * @param {WheelRenderRequest} props
 */
export const WheelComposition = (props) => {
  const frame = useCurrentFrame();
  const {durationInFrames, fps, width, height} = useVideoConfig();
  const theme = getTheme(props.theme);
  const spinFrames = Math.max(1, durationInFrames - Math.round((props.holdMs / 1000) * fps));
  const holdStartFrame = spinFrames;
  const settle = interpolate(frame, [holdStartFrame, durationInFrames - 1], [0, 1], {
    easing: Easing.out(Easing.sin),
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  });
  const winnerPulse = frame >= holdStartFrame ? Math.sin(settle * Math.PI) : 0;

  const size = Math.min(width, height);
  const center = size / 2;
  const radius = size * 0.41;
  const centerRingRadius = size * 0.09;
  const centerRadius = size * 0.06;
  const sliceDegrees = getSliceDegrees(props.options.length);
  const finalRotation = getFinalWheelRotationDegrees(props.options.length, props.winnerIndex);
  const startRotation = -24;
  const rotation = getWheelRotationDegrees({
    finalRotationDegrees: finalRotation,
    frame,
    spinFrames,
    startRotationDegrees: startRotation,
  });

  return (
    <AbsoluteFill
      style={{
        alignItems: 'center',
        background: theme.background,
        display: 'flex',
        fontFamily: baseFont,
        justifyContent: 'center',
      }}
    >
      <div
        style={{
          height: size,
          position: 'relative',
          width: size,
        }}
      >
        <div
          style={{
            borderRadius: '50%',
            boxShadow: theme.wheelShadow,
            height: radius * 2,
            left: center - radius,
            position: 'absolute',
            top: center - radius,
            width: radius * 2,
          }}
        />

        <div
          style={{
            height: size,
            left: 0,
            position: 'absolute',
            top: 0,
            transform: `rotate(${rotation}deg)`,
            transformOrigin: '50% 50%',
            width: size,
          }}
        >
          {props.options.map((option, index) => {
            const start = index * sliceDegrees;
            const centerDegrees = getSliceCenterDegrees(props.options.length, index);
            const isWinner = index === props.winnerIndex;
            const color = theme.slices[index % theme.slices.length] ?? theme.slices[0] ?? '#cccccc';
            const label = getRadialLabelLayout({
              center,
              radius,
              hubRadius: centerRingRadius,
              centerDegrees,
              optionCount: props.options.length,
              text: option,
            });

            return (
              <React.Fragment key={`${option}-${index}`}>
                <Pie
                  fill={color}
                  progress={1 / props.options.length}
                  radius={radius}
                  rotation={cssDegreesToPieRadians(start)}
                  stroke="#ffffff"
                  strokeWidth={2}
                  style={{
                    filter: isWinner && frame >= holdStartFrame ? `drop-shadow(0 0 ${10 + winnerPulse * 10}px ${theme.winnerGlow})` : undefined,
                    left: center - radius,
                    position: 'absolute',
                    top: center - radius,
                  }}
                />
                <div
                  style={{
                    alignItems: 'center',
                    boxSizing: 'border-box',
                    color: theme.text,
                    display: 'flex',
                    fontSize: label.fontSize,
                    flexDirection: 'column',
                    fontWeight: 700,
                    height: label.height,
                    justifyContent: 'center',
                    left: label.x,
                    letterSpacing: 0,
                    lineHeight: 1,
                    overflow: 'hidden',
                    padding: `0 ${label.horizontalPadding}px`,
                    position: 'absolute',
                    textAlign: 'center',
                    textShadow: `0 1px 0 ${theme.textHalo}, 0 -1px 0 ${theme.textHalo}, 1px 0 0 ${theme.textHalo}, -1px 0 0 ${theme.textHalo}`,
                    top: label.y,
                    transform: `translate(-50%, -50%) rotate(${label.rotation}deg)`,
                    transformOrigin: '50% 50%',
                    width: label.width,
                  }}
                >
                  {label.lines.map((line, lineIndex) => (
                    <div key={`${line}-${lineIndex}`} style={{flex: '0 0 auto', whiteSpace: 'nowrap'}}>
                      {line}
                    </div>
                  ))}
                </div>
              </React.Fragment>
            );
          })}
        </div>

        <Circle
          fill={theme.centerRing}
          radius={centerRingRadius}
          style={{left: center - centerRingRadius, position: 'absolute', top: center - centerRingRadius}}
        />
        <Circle
          fill={theme.center}
          radius={centerRadius}
          style={{left: center - centerRadius, position: 'absolute', top: center - centerRadius}}
        />
        <div
          style={{
            background: theme.pointer,
            border: `2px solid ${theme.pointerStroke}`,
            boxShadow: '0 7px 16px rgba(15, 23, 42, 0.24)',
            clipPath: rightPointerClipPath,
            height: size * 0.085,
            left: center + radius - size * 0.02,
            position: 'absolute',
            top: center - size * 0.0425,
            width: size * 0.105,
            zIndex: 4,
          }}
        />
      </div>

      <Confetti colors={theme.slices} seed={props.winnerIndex} startFrame={holdStartFrame} />
    </AbsoluteFill>
  );
};
