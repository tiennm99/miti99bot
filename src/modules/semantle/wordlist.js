/**
 * @file Curated target-word pool for the semantle module.
 *
 * ConceptNet has no random-word endpoint, so we ship a hand-picked list of
 * common, game-friendly English nouns/verbs/adjectives (4–10 ASCII letters).
 * The list is small on purpose — every entry is a reasonable Semantle target,
 * which matters more than raw size. Expand freely as the game matures.
 *
 * Entries are lowercase and alphabetic only (the similarity endpoint accepts
 * these directly as `/c/en/<word>` concept IDs).
 */

// biome-ignore format: keep the list compact and grep-friendly
export const TARGET_POOL = [
  // nature / geography
  "ocean", "mountain", "forest", "desert", "river", "valley", "garden", "island",
  "beach", "cave", "meadow", "glacier", "volcano", "canyon", "jungle", "lake",
  "hill", "plateau", "cliff", "harbor", "coast", "swamp", "prairie", "tundra",
  "delta", "creek", "stream", "pebble", "boulder", "horizon", "iceberg", "dune",
  // weather / time
  "winter", "summer", "autumn", "spring", "morning", "evening", "midnight",
  "sunrise", "sunset", "thunder", "rainbow", "blizzard", "breeze", "drought",
  "storm", "shadow", "twilight", "decade",
  // people / relations
  "friend", "family", "mother", "father", "brother", "sister", "child",
  "stranger", "neighbor", "partner", "sibling", "elder", "infant",
  // arts
  "music", "dance", "poem", "story", "painting", "theater", "cinema", "novel",
  "symphony", "sculpture", "sketch", "ballet", "opera", "concert",
  // objects
  "computer", "phone", "camera", "robot", "engine", "wheel", "pencil", "hammer",
  "mirror", "bicycle", "umbrella", "lantern", "compass", "anchor", "blanket",
  "candle", "cushion", "kettle", "ladder", "needle", "paper", "pillow",
  "scissors", "telescope", "throne", "vase", "window", "zipper", "bottle",
  "basket", "bridge", "tower",
  // animals
  "eagle", "tiger", "dolphin", "rabbit", "snake", "salmon", "wolf", "horse",
  "butterfly", "elephant", "panda", "falcon", "sparrow", "penguin", "octopus",
  "beetle", "crow", "dragon", "hawk", "jaguar", "kangaroo", "lion", "monkey",
  "otter", "parrot", "raccoon", "squirrel", "turtle", "whale", "bear", "fox",
  "shark",
  // food
  "apple", "bread", "cheese", "coffee", "sugar", "pepper", "potato", "orange",
  "honey", "chocolate", "cinnamon", "almond", "berry", "butter", "grape",
  "lemon", "olive", "tomato", "walnut", "yogurt", "ginger",
  // emotions / abstract
  "love", "anger", "fear", "courage", "sorrow", "wonder", "dream", "memory",
  "silence", "laughter", "hope", "delight", "regret", "trust", "justice",
  "freedom", "honor", "peace", "victory", "promise", "secret", "truth",
  "wisdom", "mystery", "destiny", "patience", "loyalty",
  // professions
  "teacher", "doctor", "artist", "farmer", "pilot", "soldier", "writer",
  "sailor", "hunter", "engineer", "chef", "dentist", "nurse", "judge",
  "scientist",
  // body
  "shoulder", "finger", "elbow", "throat", "pulse", "tongue", "ankle", "spine",
  "muscle", "nerve", "eyelid", "beard", "tooth", "thumb", "knee",
  // verbs / actions
  "gather", "wonder", "linger", "stumble", "whisper", "shimmer", "wander",
  "rescue", "gallop", "imagine", "pursue", "retreat", "thrive", "squeeze",
  "shatter", "tremble", "travel", "borrow", "descend", "inherit", "vanish",
  "forget", "invite", "resist", "settle",
  // adjectives
  "ancient", "gentle", "fragile", "massive", "glowing", "rugged", "silent",
  "frozen", "steady", "brittle", "clever", "gloomy", "cheerful", "noisy",
  "silver", "golden", "radiant", "distant", "graceful", "humble", "vivid",
  "quiet", "sacred", "sudden", "tender", "wealthy", "generous", "majestic",
];

/**
 * @returns {string} — a random lowercase word from the pool.
 */
export function pickFromPool() {
  return TARGET_POOL[Math.floor(Math.random() * TARGET_POOL.length)];
}
