/**
 * @typedef {object} LabelTrack
 * @property {number} startRadius
 * @property {number} endRadius
 * @property {number} midRadius
 * @property {number} width
 */

/**
 * @typedef {object} RadialLabelLayout
 * @property {number} x
 * @property {number} y
 * @property {number} width
 * @property {number} rotation
 * @property {number} fontSize
 */

const minLabelWidth = 24;

/**
 * @param {string} text
 * @param {number} fontSize
 * @returns {number}
 */
export const estimateTextWidth = (text, fontSize) => {
  const widthUnits = Array.from(text).reduce((sum, char) => {
    if (/[\s.,'`|!il:;]/u.test(char)) {
      return sum + 0.32;
    }

    if (/[A-ZMW@#%&]/u.test(char)) {
      return sum + 0.72;
    }

    return sum + 0.56;
  }, 0);

  return (widthUnits + 2) * fontSize;
};

/**
 * @param {number} radius
 * @param {number} hubRadius
 * @returns {LabelTrack}
 */
export const getLabelTrack = (radius, hubRadius) => {
  const startRadius = hubRadius + Math.max(8, radius * 0.08);
  const endRadius = radius - Math.max(5, radius * 0.035);
  const width = Math.max(minLabelWidth, endRadius - startRadius);

  return {
    startRadius,
    endRadius: startRadius + width,
    midRadius: startRadius + width / 2,
    width,
  };
};

/**
 * @param {number} radius
 * @param {number} optionCount
 * @param {string} text
 * @param {number} trackWidth
 * @returns {number}
 */
export const getLabelFontSize = (radius, optionCount, text = '', trackWidth = Number.POSITIVE_INFINITY) => {
  const baseSize = Math.max(10, Math.min(24, radius / Math.max(5.8, optionCount * 0.33)));

  if (!text || !Number.isFinite(trackWidth)) {
    return baseSize;
  }

  const estimatedWidth = estimateTextWidth(text, baseSize);
  if (estimatedWidth <= trackWidth) {
    return baseSize;
  }

  return Math.max(8, Math.floor(baseSize * (trackWidth / estimatedWidth)));
};

/**
 * @param {object} input
 * @param {number} input.center
 * @param {number} input.radius
 * @param {number} input.hubRadius
 * @param {number} input.centerDegrees
 * @param {number} input.optionCount
 * @param {string} input.text
 * @returns {RadialLabelLayout}
 */
export const getRadialLabelLayout = ({center, radius, hubRadius, centerDegrees, optionCount, text}) => {
  const track = getLabelTrack(radius, hubRadius);
  const radians = (centerDegrees * Math.PI) / 180;

  return {
    x: center + Math.cos(radians) * track.midRadius,
    y: center + Math.sin(radians) * track.midRadius,
    width: track.width,
    rotation: centerDegrees,
    fontSize: getLabelFontSize(radius, optionCount, text, track.width),
  };
};

/**
 * @param {string} color
 * @returns {'#111827' | '#ffffff'}
 */
export const getContrastingTextColor = (color) => {
  const match = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/iu.exec(color);
  if (!match) {
    return '#111827';
  }

  const redHex = match[1] ?? '00';
  const greenHex = match[2] ?? '00';
  const blueHex = match[3] ?? '00';
  const red = Number.parseInt(redHex, 16) / 255;
  const green = Number.parseInt(greenHex, 16) / 255;
  const blue = Number.parseInt(blueHex, 16) / 255;
  const luminance = 0.2126 * red + 0.7152 * green + 0.0722 * blue;

  return luminance > 0.56 ? '#111827' : '#ffffff';
};
