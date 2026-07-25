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
 * @property {number} contentWidth
 * @property {number} height
 * @property {number} horizontalPadding
 * @property {number} rotation
 * @property {number} fontSize
 * @property {string[]} lines
 */

const minLabelWidth = 24;
const minLabelFontSize = 8;
const preferredLabelFontSize = 14;

/**
 * Line box as a multiple of the font size.
 *
 * Vietnamese stacks a tone mark above and a dot below the same vowel, so a
 * ratio of exactly 1 leaves marks from adjacent wrapped lines touching, and any
 * font fallback with taller metrics clips against the label's `overflow:
 * hidden`. Exported so the composition renders the same box this module sizes.
 */
export const labelLineHeightRatio = 1.16;

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
 * @param {string[]} units
 * @param {number} lineCount
 * @param {(units: string[]) => string} joinUnits
 * @returns {string[][]}
 */
const getLineCandidates = (units, lineCount, joinUnits) => {
  /** @type {string[][]} */
  const candidates = [];

  /**
   * @param {number} start
   * @param {string[]} lines
   */
  const visit = (start, lines) => {
    const remainingLines = lineCount - lines.length;
    if (remainingLines === 1) {
      candidates.push([...lines, joinUnits(units.slice(start))]);
      return;
    }

    const lastEnd = units.length - remainingLines + 1;
    for (let end = start + 1; end <= lastEnd; end += 1) {
      visit(end, [...lines, joinUnits(units.slice(start, end))]);
    }
  };

  visit(0, []);
  return candidates;
};

/**
 * @param {string[][]} candidates
 * @param {number} fontSize
 * @returns {string[]}
 */
const getMostBalancedLines = (candidates, fontSize) => {
  const scored = candidates.map((lines, index) => {
    const widths = lines.map((line) => estimateTextWidth(line, fontSize));
    const longest = Math.max(...widths);
    const shortest = Math.min(...widths);

    return {index, lines, longest, spread: longest - shortest};
  });

  scored.sort((left, right) => left.longest - right.longest || left.spread - right.spread || left.index - right.index);
  return scored[0]?.lines ?? [''];
};

/**
 * Selects deterministic line breaks without depending on browser text wrapping.
 *
 * @param {string} text
 * @param {number} fontSize
 * @param {number} trackWidth
 * @param {number} [maxLines]
 * @returns {string[]}
 */
export const getLabelLines = (text, fontSize, trackWidth, maxLines = 3) => {
  const normalized = text.trim().replace(/\s+/gu, ' ');
  if (!normalized || estimateTextWidth(normalized, fontSize) <= trackWidth) {
    return [normalized];
  }

  const words = normalized.split(' ');
  const boundedMaxLines = Math.max(1, Math.min(Math.floor(maxLines), words.length > 1 ? words.length : Number.POSITIVE_INFINITY));
  const segmenter = new Intl.Segmenter('en', {granularity: 'grapheme'});
  const units = words.length > 1 ? words : Array.from(segmenter.segment(normalized), ({segment}) => segment);
  /** @type {(parts: string[]) => string} */
  const joinUnits = words.length > 1 ? (parts) => parts.join(' ') : (parts) => parts.join('');
  const lineLimit = Math.max(1, Math.min(boundedMaxLines, units.length));
  let bestLines = [normalized];

  for (let lineCount = 2; lineCount <= lineLimit; lineCount += 1) {
    const candidates = getLineCandidates(units, lineCount, joinUnits);
    bestLines = getMostBalancedLines(candidates, fontSize);
    if (Math.max(...bestLines.map((line) => estimateTextWidth(line, fontSize))) <= trackWidth) {
      return bestLines;
    }
  }

  return bestLines;
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

  return Math.max(minLabelFontSize, Math.floor(baseSize * (trackWidth / estimatedWidth)));
};

/**
 * @param {object} input
 * @param {number} input.center
 * @param {number} input.radius
 * @param {number} input.hubRadius
 * @param {number} input.centerDegrees
 * @param {number} input.optionCount
 * @param {string} input.text
 * @param {number} [input.winnerCenterDegrees] center angle of the winning slice
 * @returns {RadialLabelLayout}
 */
export const getRadialLabelLayout = ({
  center,
  radius,
  hubRadius,
  centerDegrees,
  optionCount,
  text,
  winnerCenterDegrees = 0,
}) => {
  const track = getLabelTrack(radius, hubRadius);
  const radians = (centerDegrees * Math.PI) / 180;
  const baseFontSize = getLabelFontSize(radius, optionCount);
  const availableHeight = (2 * Math.PI * track.midRadius) / optionCount;
  // Counted against the bare font size, not the line box: the fitted size below
  // shrinks the text if the taller boxes overflow, so charging the ratio here
  // too would drop a line that still fits once shrunk.
  const maxLines = Math.max(1, Math.min(3, Math.floor(availableHeight / baseFontSize)));
  const horizontalPadding = Math.max(4, radius * 0.02);
  const contentWidth = Math.max(1, track.width - horizontalPadding * 2);
  const singleLine = getLabelLines(text, baseFontSize, contentWidth, 1);
  const singleLineFontSize = getLabelFontSize(radius, optionCount, singleLine[0], contentWidth);
  const shouldWrap = maxLines > 1 && singleLineFontSize < preferredLabelFontSize;
  const lines = shouldWrap ? getLabelLines(text, baseFontSize, contentWidth, maxLines) : singleLine;
  const longestLine = lines.reduce(
    (longest, line) =>
      estimateTextWidth(line, baseFontSize) > estimateTextWidth(longest, baseFontSize) ? line : longest,
    '',
  );
  const fontSize = shouldWrap
    ? getLabelFontSize(radius, optionCount, longestLine, contentWidth)
    : singleLineFontSize;
  // Shrink to fit the taller line box rather than letting `overflow: hidden`
  // crop the last line.
  const blockHeight = fontSize * labelLineHeightRatio * lines.length;
  const fittedFontSize =
    blockHeight > availableHeight
      ? Math.max(minLabelFontSize, Math.floor(fontSize * (availableHeight / blockHeight)))
      : fontSize;
  // Labels stay radial, but a slice whose rest-frame angle points left would
  // render its text mirrored. Flipping by the angle the slice lands at makes
  // every label upright in the final frame, which is the frame chat clients
  // show as the GIF's poster image.
  //
  // The boundary is inclusive at 90 and exclusive at 270 so the two labels that
  // land exactly vertical — which happens on every option count divisible by
  // four — both read in the same direction. Neither is mirrored either way, but
  // opposite reading directions make the pair look like a mistake.
  const restAngle = (((centerDegrees - winnerCenterDegrees) % 360) + 360) % 360;
  const flip = restAngle >= 90 && restAngle < 270 ? 180 : 0;

  return {
    x: center + Math.cos(radians) * track.midRadius,
    y: center + Math.sin(radians) * track.midRadius,
    width: track.width,
    contentWidth,
    height: Math.min(availableHeight, fittedFontSize * labelLineHeightRatio * lines.length),
    horizontalPadding,
    rotation: centerDegrees + flip,
    fontSize: fittedFontSize,
    lines,
  };
};
