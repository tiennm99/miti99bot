// Browser page for the gacha wish: one pack-cards pack whose single card shows
// the rolled label, rank, and stars. The renderer injects `window.gachaWish`
// before this module runs, tears the pack open with a scripted drag, and
// captures each frame.
import {createPackView} from 'pack-cards';

/**
 * @typedef {object} GachaWishPage
 * @property {string} label
 * @property {string} rank       Rank letter, as on /api/gacha.
 * @property {number} stars
 * @property {string} rarity     pack-cards rarity profile.
 * @property {'silver' | 'platinum' | 'gold'} glow  pack-cards glow preset.
 * @property {{accent: string, tint: string}} artwork
 */

const wish = /** @type {GachaWishPage} */ (/** @type {any} */ (window).gachaWish);
const stage = /** @type {HTMLElement} */ (document.getElementById('stage'));
stage.style.setProperty('--wish-accent', wish.artwork.accent);
stage.style.setProperty('--wish-tint', wish.artwork.tint);

/**
 * @param {string} tag
 * @param {string} className
 * @param {string} [text]
 */
const el = (tag, className, text) => {
  const node = document.createElement(tag);
  node.className = className;
  if (text !== undefined) {
    node.textContent = text;
  }
  return node;
};

const createFace = () => {
  const face = el('article', 'wish-card');
  const header = el('header', 'recap-card-header');
  header.append(el('span', '', 'GACHA'), el('span', '', `RANK ${wish.rank}`));
  const content = el('div', 'recap-card-content');
  const title = el('h2', 'wish-label', wish.label);
  title.setAttribute('data-foil-text', '');
  content.append(el('span', 'wish-rank', wish.rank), title, el('p', 'wish-stars', '★'.repeat(wish.stars)));
  const footer = el('footer', 'recap-card-footer');
  footer.append(el('span', '', `${wish.stars}★`), el('span', '', '01 / 01'));
  face.append(header, content, footer);
  return face;
};

/**
 * pack-cards (pinned commit 83e8fbb) flies the revealed card out of the pack
 * over 1750ms: the card is clear of the pack at 900ms, then it flips with a
 * half turn (rotateY 180deg to 0deg) and the pack drops away while it glides
 * to the centre. The page reshapes those animations without changing the
 * library: the rise out of the pack eases exponentially in and out,
 * everything after the card clears the pack is stretched so the flip has time
 * for three extra turns, and the flip eases out like a flicked card slowing to
 * a stop while sparkles burst around it.
 */
const flight = Object.freeze({duration: 1750, clearAt: 900});
/** Exponential ease-in-out, as a CSS timing function. */
const easeInOutExpo = 'cubic-bezier(.87, 0, .13, 1)';
const afterClearStretch = 1.65;
const flip = Object.freeze({startDegrees: 180 + 3 * 360, easing: 'cubic-bezier(.3, .4, .25, 1)'});

/**
 * Maps a time in the original flight to the stretched one.
 *
 * @param {number} ms
 */
const stretch = (ms) => (ms <= flight.clearAt ? ms : flight.clearAt + (ms - flight.clearAt) * afterClearStretch);

const sparkleColors = [wish.artwork.accent, '#ffffff', '#fff0b8'];

/**
 * Bursts small four-point sparkles out from the card while it spins. They
 * sit beside the turning face inside the card's deck, so they travel with
 * the card without spinning.
 *
 * @param {Element} deck
 * @param {number} delay     Milliseconds until the spin starts.
 * @param {number} duration  Length of the spin.
 */
const burstSparkles = (deck, delay, duration) => {
  const layer = el('div', 'wish-sparkles');
  deck.append(layer);
  const size = deck.getBoundingClientRect().width || 200;
  for (let index = 0; index < 72; index += 1) {
    const sparkle = el('span', 'wish-sparkle');
    const angle = Math.random() * Math.PI * 2;
    const reach = size * (0.55 + Math.random() * 0.75);
    const scale = 0.6 + Math.random() * 0.9;
    sparkle.style.setProperty('--sparkle', sparkleColors[index % sparkleColors.length] ?? '#ffffff');
    const dx = Math.cos(angle) * reach;
    const dy = Math.sin(angle) * reach * 1.3;
    sparkle.animate(
      [
        {transform: 'translate(-50%, -50%) scale(0) rotate(0deg)', opacity: 0},
        {
          transform: `translate(calc(-50% + ${dx * 0.45}px), calc(-50% + ${dy * 0.45}px)) scale(${scale}) rotate(45deg)`,
          opacity: 1,
          offset: 0.25,
        },
        {transform: `translate(calc(-50% + ${dx}px), calc(-50% + ${dy}px)) scale(0) rotate(90deg)`, opacity: 0},
      ],
      {
        // Squaring front-loads the burst as the spin begins, then it thins out.
        delay: delay + Math.random() ** 2 * duration * 0.8,
        duration: 500 + Math.random() * 500,
        easing: 'cubic-bezier(.2, .7, .3, 1)',
        fill: 'both',
      },
    );
    layer.append(sparkle);
  }
  setTimeout(() => layer.remove(), delay + duration + 1200);
};

/**
 * Engravings carry the polychrome (page.css): 4★ cards are tooled with the
 * epic contour, 5★ cards with the legendary rings or facets, picked by
 * pack-cards from the card's label. 3★ cards stay plain.
 *
 * @type {import('pack-cards').AppearanceSettingsOptions['rarities']}
 */
const engravings = Object.freeze({
  epic: {engraving: 'contour'},
  legendary: {engraving: ['radial', 'facets']},
});

const animate = Element.prototype.animate;
/**
 * @this {Element}
 * @param {Keyframe[] | PropertyIndexedKeyframes | null} keyframes
 * @param {number | KeyframeAnimationOptions} [options]
 */
Element.prototype.animate = function (keyframes, options) {
  if (!Array.isArray(keyframes) || typeof options !== 'object') {
    return animate.call(this, keyframes, options);
  }
  // The flight path: keyframes spaced evenly over the whole flight.
  if (options.duration === flight.duration && options.easing === 'linear') {
    const total = stretch(flight.duration);
    const at = (/** @type {Keyframe} */ frame) => Number(frame.offset) * flight.duration;
    // Until the card clears the pack it only translates, so the rise is one
    // straight segment: keep its two ends and ease between them.
    const cleared = keyframes.findIndex((frame) => at(frame) > flight.clearAt);
    const [start] = keyframes;
    const eased =
      start && cleared > 1 ? [{...start, easing: easeInOutExpo}, ...keyframes.slice(cleared - 1)] : keyframes;
    const stretched = eased.map((frame) => ({
      ...frame,
      offset: stretch(at(frame)) / total,
    }));
    return animate.call(this, stretched, {...options, duration: total});
  }
  // The flip, the pack dropping away, and the spreading backs all start as the card clears the pack.
  if (options.delay === flight.clearAt && options.duration === flight.duration - flight.clearAt) {
    const duration = stretch(flight.duration) - flight.clearAt;
    const isFlip =
      keyframes.length === 2 &&
      keyframes[0]?.transform === 'rotateY(180deg)' &&
      keyframes[1]?.transform === 'rotateY(0deg)';
    if (!isFlip) {
      return animate.call(this, keyframes, {...options, duration});
    }
    if (this.parentElement) {
      burstSparkles(this.parentElement, flight.clearAt, duration);
    }
    return animate.call(this, [{transform: `rotateY(${flip.startDegrees}deg)`}, keyframes[1] ?? {}], {
      ...options,
      duration,
      easing: flip.easing,
    });
  }
  return animate.call(this, keyframes, options);
};

const view = createPackView(stage, {
  label: 'Gacha',
  artwork: wish.artwork,
  labels: {title: 'Gacha', swipe: '', tap: ''},
  appearance: {opening: 'animated', motion: 'interactive', glow: wish.glow, pack_zoom: true, rarities: engravings},
});

view.showPack({
  count: 1,
  renderCard: () => ({face: createFace(), identity: wish.label, rarity: wish.rarity}),
  onReveal: () => {
    document.documentElement.dataset.wish = 'revealed';
  },
  onError: (error) => {
    document.documentElement.dataset.wish = 'failed';
    console.error(error);
  },
});
