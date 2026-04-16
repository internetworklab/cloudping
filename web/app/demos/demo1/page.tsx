"use client";

import { Box } from "@mui/material";
import { CSSProperties, useEffect, useRef, useState } from "react";

type MasonryItem = {
  title: string;
  description: string;
  imageUrl: string;
  paper?: {
    height: number;
    color: CSSProperties["color"];
  };
};

interface CardProps {
  title: string;
  description: string;
  imageUrl: string;
  paper?: {
    height: number;
    color: CSSProperties["color"];
  };
}

function Card(props: CardProps) {
  return (
    <Box
      sx={{
        position: "relative",
        overflow: "hidden",
        borderRadius: 2,
        cursor: "pointer",
        fontFamily: "sans-serif",
        color: "white",
        width: "100%",
        height: props.paper ? props.paper.height + "px" : "auto",
        "&:hover .card-description": {
          WebkitLineClamp: "unset",
          overflow: "visible",
        },
      }}
    >
      {props.paper ? (
        <Box
          sx={{
            width: "100%",
            height: "100%",
            backgroundColor: props.paper.color,
          }}
        />
      ) : (
        <Box
          component="img"
          src={props.imageUrl}
          alt={props.title}
          sx={{
            display: "block",
            width: "100%",
            height: "auto",
            objectFit: "cover",
          }}
        />
      )}

      <Box
        sx={{
          position: "absolute",
          top: 0,
          left: 0,
          width: "100%",
          height: "100%",
          background:
            "linear-gradient(to bottom, rgba(0,0,0,0.05) 0%, rgba(0,0,0,0.25) 35%, rgba(0,0,0,0.70) 70%, rgba(0,0,0,0.95) 100%)",
          zIndex: 2,
          pointerEvents: "none",
        }}
      />

      <Box
        sx={{
          position: "absolute",
          bottom: 0,
          p: "16px",
          zIndex: 3,
          width: "100%",
          boxSizing: "border-box",
        }}
      >
        <Box
          component="h2"
          sx={{
            margin: "0 0 8px 0",
            fontSize: "1.25rem",
            fontWeight: "bold",
          }}
        >
          {props.title}
        </Box>
        <Box
          component="p"
          className="card-description"
          sx={{
            margin: 0,
            fontSize: "0.9rem",
            lineHeight: 1.4,
            display: "-webkit-box",
            WebkitLineClamp: 2,
            WebkitBoxOrient: "vertical",
            overflow: "hidden",
            textOverflow: "ellipsis",
            transition: "all 0.3s ease-in-out",
          }}
        >
          {props.description}
        </Box>
      </Box>
    </Box>
  );
}

const cards: MasonryItem[] = [
  {
    title: "Cyberpunk Streets",
    description:
      "This is a very long description that will eventually overflow the container because it contains a lot of unnecessary detail about neon lights, rain-slicked pavement, and futuristic technology.",
    imageUrl: "https://picsum.photos/id/104/800/600",
    paper: {
      color: "#f59e0b",
      height: 320,
    },
  },
  {
    title: "Mountain Serenity",
    description:
      "A breathtaking view of snow-capped mountains reflected in a crystal-clear alpine lake, surrounded by towering pines and the quiet stillness of nature at dawn.",
    imageUrl: "https://picsum.photos/id/29/600/900",
  },
  {
    title: "Urban Jungle",
    description:
      "The towering skyscrapers of the downtown core pierce through low-hanging clouds, their glass facades catching the golden hues of a spectacular sunset over the city skyline.",
    imageUrl: "https://picsum.photos/id/149/800/500",
  },
  {
    title: "Coastal Escape",
    description:
      "Turquoise waves crash against weathered limestone cliffs as seabirds circle overhead, painting a picture of untamed coastal beauty that has drawn travelers for centuries.",
    imageUrl: "https://picsum.photos/id/165/700/1000",
  },
  {
    title: "Autumn Whispers",
    description:
      "A winding forest path blanketed in amber and crimson leaves, with sunlight filtering through the canopy creating dappled patterns on the ground below.",
    imageUrl: "https://picsum.photos/id/135/800/600",
  },
  {
    title: "Desert Mirage",
    description:
      "Endless golden sand dunes stretch to the horizon under a blazing sunset, their curved ridges casting dramatic shadows that shift with each passing hour.",
    imageUrl: "https://picsum.photos/id/25/800/450",
    paper: {
      color: "#0ea5e9",
      height: 260,
    },
  },
  {
    title: "Neon Nights",
    description:
      "Rain-slicked streets reflect the glow of neon signs in a bustling downtown district, where the energy of the city never seems to fade.",
    imageUrl: "https://picsum.photos/id/122/600/800",
  },
  {
    title: "Quiet Library",
    description:
      "Rows of ancient books line mahogany shelves in a grand reading room bathed in warm amber light from towering arched windows.",
    imageUrl: "https://picsum.photos/id/24/800/600",
  },
  {
    title: "Vintage Ride",
    description:
      "A classic convertible parked along a sun-drenched coastal highway, its chrome details gleaming against the backdrop of an endless blue sky.",
    imageUrl: "https://picsum.photos/id/111/700/900",
  },
  {
    title: "Foggy Morning",
    description:
      "A lone figure walks across a wooden bridge shrouded in thick morning fog, the world reduced to soft greys and muted silence.",
    imageUrl: "https://picsum.photos/id/118/800/500",
    paper: {
      color: "#8b5cf6",
      height: 380,
    },
  },
  {
    title: "Street Food",
    description:
      "Steam rises from sizzling woks at a bustling night market, where vendors serve up generations-old recipes to eager crowds.",
    imageUrl: "https://picsum.photos/id/225/600/700",
  },
  {
    title: "Arctic Glow",
    description:
      "The aurora borealis dances across a star-filled sky in brilliant curtains of green and violet over a frozen landscape.",
    imageUrl: "https://picsum.photos/id/54/800/600",
  },
  {
    title: "Ocean Depths",
    description:
      "Sunlight pierces through crystal-clear turquoise water, illuminating a vibrant coral reef teeming with colorful tropical fish.",
    imageUrl: "https://picsum.photos/id/124/700/900",
  },
  {
    title: "Crimson Horizon",
    description:
      "A bold, warm canvas that evokes the energy of a setting sun over a vast desert plain.",
    imageUrl: "https://picsum.photos/id/124/700/900",
    paper: {
      color: "#ef4444",
      height: 290,
    },
  },
  {
    title: "Lavender Fields",
    description:
      "Endless rows of fragrant lavender sway in a gentle Provence breeze, their soft purple hues stretching toward the distant Alps.",
    imageUrl: "https://picsum.photos/id/152/800/500",
  },
  {
    title: "Concrete Canvas",
    description:
      "A minimalist expanse of cool grey, inspired by raw urban architecture and modern design sensibilities.",
    imageUrl: "https://picsum.photos/id/152/800/500",
    paper: {
      color: "#64748b",
      height: 340,
    },
  },
  {
    title: "Golden Harvest",
    description:
      "Rolling wheat fields glow amber under a late summer sun, promising abundance and the simple beauty of rural life.",
    imageUrl: "https://picsum.photos/id/112/800/600",
  },
  {
    title: "Jungle Canopy",
    description:
      "Thick vines and lush emerald foliage create a living cathedral where sunlight barely reaches the forest floor.",
    imageUrl: "https://picsum.photos/id/1023/600/800",
  },
  {
    title: "Mint Breeze",
    description:
      "A refreshing wash of cool mint green, like the first breath of spring after a long winter.",
    imageUrl: "https://picsum.photos/id/1023/600/800",
    paper: {
      color: "#14b8a6",
      height: 270,
    },
  },
  {
    title: "Midnight City",
    description:
      "A panoramic rooftop view of a sprawling metropolis glittering beneath a crescent moon and velvet sky.",
    imageUrl: "https://picsum.photos/id/1076/800/600",
  },
  {
    title: "Blush Rose",
    description:
      "Soft, romantic pink tones that recall petals unfurling at dawn and the quiet joy of a garden in bloom.",
    imageUrl: "https://picsum.photos/id/1076/800/600",
    paper: {
      color: "#f472b6",
      height: 310,
    },
  },
  {
    title: "Starry Wilderness",
    description:
      "The Milky Way arches over a silent canyon, countless stars painting the darkness with ancient light.",
    imageUrl: "https://picsum.photos/id/974/700/1000",
  },
  {
    title: "Citrus Burst",
    description:
      "A zesty, energetic splash of orange that radiates warmth, creativity, and sunshine.",
    imageUrl: "https://picsum.photos/id/974/700/1000",
    paper: {
      color: "#f97316",
      height: 250,
    },
  },
  {
    title: "Frozen Lake",
    description:
      "A pristine alpine lake frozen solid beneath a blanket of fresh powder snow, surrounded by silent pines and jagged peaks.",
    imageUrl: "https://picsum.photos/id/971/800/600",
  },
  {
    title: "Electric Blue",
    description:
      "A shock of vibrant cobalt that pulses with digital energy and the hum of a futuristic skyline.",
    imageUrl: "https://picsum.photos/id/971/800/600",
    paper: {
      color: "#3b82f6",
      height: 300,
    },
  },
  {
    title: "Sakura Season",
    description:
      "Delicate cherry blossom petals drift on a gentle wind, blanketing a quiet Kyoto canal in a sea of pale pink.",
    imageUrl: "https://picsum.photos/id/106/700/900",
  },
  {
    title: "Volcanic Earth",
    description:
      "Rich, deep umber tones drawn from cooling lava fields and the raw power of the planet's core.",
    imageUrl: "https://picsum.photos/id/106/700/900",
    paper: {
      color: "#78350f",
      height: 280,
    },
  },
  {
    title: "Rainforest Mist",
    description:
      "Morning fog rolls through dense tropical undergrowth, moisture beading on enormous leaves in a symphony of greens.",
    imageUrl: "https://picsum.photos/id/1020/800/500",
  },
  {
    title: "Highland Trek",
    description:
      "A lone hiker follows a narrow ridge trail above a sea of clouds, the rugged highlands stretching to every horizon.",
    imageUrl: "https://picsum.photos/id/1018/800/600",
  },
  {
    title: "Solar Flare",
    description:
      "Brilliant, blazing yellow that captures the unfiltered intensity of the midday sun.",
    imageUrl: "https://picsum.photos/id/1018/800/600",
    paper: {
      color: "#eab308",
      height: 320,
    },
  },
  {
    title: "Venice Twilight",
    description:
      "Gondolas glide past ancient palazzos as the last light of day turns the canals into liquid gold.",
    imageUrl: "https://picsum.photos/id/164/600/800",
  },
  {
    title: "Deep Indigo",
    description:
      "A midnight navy so deep it feels infinite, like staring into the open ocean beneath a moonless sky.",
    imageUrl: "https://picsum.photos/id/164/600/800",
    paper: {
      color: "#312e81",
      height: 260,
    },
  },
  {
    title: "Wildflower Meadow",
    description:
      "Countless wildflowers in every shade imaginable blanket an alpine meadow, buzzing with bees and fluttering butterflies.",
    imageUrl: "https://picsum.photos/id/137/800/600",
  },
  {
    title: "Rustic Barn",
    description:
      "A weathered red barn stands against a dramatic stormy sky, its paint peeling to reveal decades of stories.",
    imageUrl: "https://picsum.photos/id/101/800/600",
  },
  {
    title: "Forest Emerald",
    description:
      "A lush, saturated green that breathes life into any space, inspired by ancient moss-covered woodlands.",
    imageUrl: "https://picsum.photos/id/101/800/600",
    paper: {
      color: "#10b981",
      height: 350,
    },
  },
  {
    title: "Desert Road",
    description:
      "A long, straight road cuts through barren desert scrub, disappearing into a shimmering heat haze on the horizon.",
    imageUrl: "https://picsum.photos/id/1072/800/500",
  },
  {
    title: "Cherry Pie",
    description:
      "Warm, inviting crimson red that smells of summer kitchens and windowsill cooling racks.",
    imageUrl: "https://picsum.photos/id/1072/800/500",
    paper: {
      color: "#be123c",
      height: 230,
    },
  },
  {
    title: "Northern Lights",
    description:
      "Vivid ribbons of green and purple light swirl above a snow-covered fjord in one of nature's grandest performances.",
    imageUrl: "https://picsum.photos/id/1022/800/600",
  },
  {
    title: "Cobblestone Alley",
    description:
      "A narrow European alley curves between centuries-old stone buildings, their balconies dripping with flowering vines.",
    imageUrl: "https://picsum.photos/id/188/600/900",
  },
  {
    title: "Peachy Keen",
    description:
      "A soft, warm peach that feels like a lazy Sunday morning and sun on your skin.",
    imageUrl: "https://picsum.photos/id/188/600/900",
    paper: {
      color: "#fdba74",
      height: 310,
    },
  },
  {
    title: "Glacier Blue",
    description:
      "Ancient ice formations glow an impossible shade of blue, compressed over millennia into translucent crystal.",
    imageUrl: "https://picsum.photos/id/1036/800/600",
  },
  {
    title: "Midnight Orchid",
    description:
      "A mysterious, velvety purple that whispers of hidden gardens and moonlit petals.",
    imageUrl: "https://picsum.photos/id/1036/800/600",
    paper: {
      color: "#7c3aed",
      height: 290,
    },
  },
  {
    title: "Thunderstorm",
    description:
      "Dark, roiling clouds unleash sheets of rain over an open prairie, lightning illuminating the sky in jagged bursts.",
    imageUrl: "https://picsum.photos/id/1021/800/500",
  },
  {
    title: "Slate Storm",
    description:
      "A moody, brooding charcoal that captures the tension of a sky just before it breaks.",
    imageUrl: "https://picsum.photos/id/1021/800/500",
    paper: {
      color: "#475569",
      height: 340,
    },
  },
  {
    title: "Tropical Paradise",
    description:
      "Pristine white sand meets impossibly clear water on a secluded island beach framed by swaying coconut palms.",
    imageUrl: "https://picsum.photos/id/12/800/600",
  },
  {
    title: "Coral Reef",
    description:
      "A lively, playful pink-orange inspired by tropical sunsets and the soft blush of sea anemones.",
    imageUrl: "https://picsum.photos/id/12/800/600",
    paper: {
      color: "#fb7185",
      height: 270,
    },
  },
];

const MIN_TRACK_WIDTH = 280;

function distributeIntoColumns(
  items: MasonryItem[],
  columnCount: number
): MasonryItem[][] {
  const columns: MasonryItem[][] = Array.from({ length: columnCount }, () => []);
  const columnHeights: number[] = new Array(columnCount).fill(0);

  for (const item of items) {
    let shortestIdx = 0;
    let shortestHeight = columnHeights[0];
    for (let i = 1; i < columnCount; i++) {
      if (columnHeights[i] < shortestHeight) {
        shortestHeight = columnHeights[i];
        shortestIdx = i;
      }
    }
    columns[shortestIdx].push(item);
    columnHeights[shortestIdx] += item.paper ? item.paper.height : 300;
  }

  return columns;
}

export default function Page() {
  const containerRef = useRef<HTMLDivElement>(null);
  const [columnCount, setColumnCount] = useState(3);

  useEffect(() => {
    const calculateColumns = () => {
      if (!containerRef.current) return;
      const containerWidth = containerRef.current.offsetWidth;
      const count = Math.max(1, Math.floor(containerWidth / MIN_TRACK_WIDTH));
      setColumnCount(count);
    };

    calculateColumns();

    const observer = new ResizeObserver(calculateColumns);
    if (containerRef.current) {
      observer.observe(containerRef.current);
    }

    return () => observer.disconnect();
  }, []);

  const columns = distributeIntoColumns(cards, columnCount);

  return (
    <Box
      ref={containerRef}
      sx={{
        width: "100%",
        boxSizing: "border-box",
        p: "20px",
        "@media (max-width: 600px)": {
          p: "10px",
        },
      }}
    >
      <Box
        sx={{
          display: "flex",
          gap: "20px",
          width: "100%",
          "@media (max-width: 600px)": {
            gap: "15px",
          },
        }}
      >
        {columns.map((column, colIdx) => (
          <Box
            key={colIdx}
            sx={{
              display: "flex",
              flexDirection: "column",
              gap: "20px",
              flex: 1,
              minWidth: 0,
              "@media (max-width: 600px)": {
                gap: "15px",
              },
            }}
          >
            {column.map((card, cardIdx) => (
              <Card key={colIdx + "-" + cardIdx} {...card} />
            ))}
          </Box>
        ))}
      </Box>
    </Box>
  );
}
