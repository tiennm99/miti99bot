/**
 * @file Seed list of secret objects for the twentyq game.
 *
 * Each seed is a hand-curated `{ category, target, initialHint }` triple.
 * The initial hint must NOT contain the target word or a close cognate —
 * it should narrow the field without giving the answer.
 *
 * Add new entries freely; tests assert hint sanity (target not in hint).
 */

/**
 * @typedef {object} Seed
 * @property {string} category
 * @property {string} target            — lowercased
 * @property {string} initialHint       — short clue, no target substring
 */

/** @type {Seed[]} */
export const SEEDS = [
  // instrument (8)
  { category: "instrument", target: "guitar", initialHint: "it has strings you pluck or strum" },
  { category: "instrument", target: "piano", initialHint: "it has black and white keys" },
  { category: "instrument", target: "drum", initialHint: "you hit it to make sound" },
  { category: "instrument", target: "violin", initialHint: "you draw a bow across its strings" },
  { category: "instrument", target: "flute", initialHint: "it uses wind to create sound" },
  {
    category: "instrument",
    target: "trumpet",
    initialHint: "brass family — buzz your lips into it",
  },
  {
    category: "instrument",
    target: "organ",
    initialHint: "it uses wind through pipes to create sound",
  },
  {
    category: "instrument",
    target: "harmonica",
    initialHint: "small enough to fit in one hand, played with the mouth",
  },

  // animal (10)
  { category: "animal", target: "elephant", initialHint: "the largest land mammal" },
  { category: "animal", target: "dolphin", initialHint: "a highly intelligent marine creature" },
  { category: "animal", target: "eagle", initialHint: "a bird of prey known for sharp eyesight" },
  {
    category: "animal",
    target: "kangaroo",
    initialHint: "this animal carries its young in a pouch",
  },
  { category: "animal", target: "octopus", initialHint: "it has eight limbs and lives in the sea" },
  {
    category: "animal",
    target: "penguin",
    initialHint: "a flightless bird that swims in cold water",
  },
  { category: "animal", target: "tiger", initialHint: "a large striped predator from Asia" },
  {
    category: "animal",
    target: "horse",
    initialHint: "domesticated for riding for thousands of years",
  },
  { category: "animal", target: "snake", initialHint: "a legless reptile" },
  {
    category: "animal",
    target: "owl",
    initialHint: "mostly nocturnal, can rotate its head far around",
  },

  // food (10)
  { category: "food", target: "pizza", initialHint: "originated in Italy, often shared in slices" },
  {
    category: "food",
    target: "sushi",
    initialHint: "a Japanese dish often featuring rice and seafood",
  },
  { category: "food", target: "burger", initialHint: "a sandwich with a patty in the middle" },
  { category: "food", target: "ramen", initialHint: "noodles served in a hot broth" },
  { category: "food", target: "taco", initialHint: "a folded shell holding savory fillings" },
  { category: "food", target: "pho", initialHint: "a Vietnamese noodle soup" },
  {
    category: "food",
    target: "curry",
    initialHint: "a heavily spiced sauce dish, popular in South Asia",
  },
  { category: "food", target: "salad", initialHint: "usually cold, often leafy and raw" },
  { category: "food", target: "chocolate", initialHint: "made from cocoa, often eaten as a treat" },
  { category: "food", target: "cheese", initialHint: "a dairy product that can be soft or hard" },

  // vehicle (8)
  { category: "vehicle", target: "bicycle", initialHint: "two wheels, powered by you" },
  { category: "vehicle", target: "car", initialHint: "the most common personal road vehicle" },
  {
    category: "vehicle",
    target: "airplane",
    initialHint: "it flies long distances carrying many people",
  },
  { category: "vehicle", target: "boat", initialHint: "it travels on water" },
  { category: "vehicle", target: "train", initialHint: "it runs on fixed metal rails" },
  {
    category: "vehicle",
    target: "motorcycle",
    initialHint: "two wheels, but powered by an engine",
  },
  { category: "vehicle", target: "helicopter", initialHint: "it flies but can hover in place" },
  { category: "vehicle", target: "submarine", initialHint: "it travels underwater" },

  // sport (8)
  { category: "sport", target: "soccer", initialHint: "the world's most popular ball sport" },
  {
    category: "sport",
    target: "basketball",
    initialHint: "you score by putting a ball through a hoop",
  },
  { category: "sport", target: "tennis", initialHint: "two or four players, a net, and rackets" },
  { category: "sport", target: "swimming", initialHint: "competed in water" },
  {
    category: "sport",
    target: "boxing",
    initialHint: "two opponents wear gloves and stand in a ring",
  },
  { category: "sport", target: "golf", initialHint: "played on grass with clubs and a small ball" },
  { category: "sport", target: "chess", initialHint: "a turn-based game on a 64-square board" },
  { category: "sport", target: "skiing", initialHint: "performed on snow" },

  // household (10)
  {
    category: "household",
    target: "refrigerator",
    initialHint: "it keeps things cold in the kitchen",
  },
  { category: "household", target: "microwave", initialHint: "it heats food using radiation" },
  { category: "household", target: "vacuum", initialHint: "it cleans floors using suction" },
  {
    category: "household",
    target: "toaster",
    initialHint: "small kitchen appliance for crisping bread",
  },
  { category: "household", target: "kettle", initialHint: "it's used to boil water" },
  {
    category: "household",
    target: "blender",
    initialHint: "it has fast-spinning blades for liquids",
  },
  { category: "household", target: "lamp", initialHint: "provides light in a room" },
  { category: "household", target: "sofa", initialHint: "you sit on it, often in the living room" },
  { category: "household", target: "mirror", initialHint: "it reflects your image" },
  { category: "household", target: "broom", initialHint: "long handle, used for sweeping" },
];

/**
 * Pick one seed at random.
 * @param {() => number} [rng]  — defaults to Math.random; tests inject deterministic rng.
 * @returns {Seed}
 */
export function getRandomSeed(rng = Math.random) {
  const i = Math.floor(rng() * SEEDS.length);
  return SEEDS[Math.min(i, SEEDS.length - 1)];
}
