import {
  ChevronDown,
  File,
  Folder,
  Image,
  LayoutGrid,
  Paperclip,
  Plus,
  RefreshCw,
  Send,
  Settings,
  Shield,
  Sparkles,
  Square,
  Wrench,
  X,
  type LucideIcon,
} from "lucide-react";

type IconName =
  | "attachment"
  | "chevron"
  | "close"
  | "file"
  | "folder"
  | "grid"
  | "image"
  | "plus"
  | "refresh"
  | "send"
  | "settings"
  | "shield"
  | "spark"
  | "stop"
  | "tool";

type IconProps = { name: IconName; className?: string };

const icons: Record<IconName, LucideIcon> = {
  attachment: Paperclip,
  chevron: ChevronDown,
  close: X,
  file: File,
  folder: Folder,
  grid: LayoutGrid,
  image: Image,
  plus: Plus,
  refresh: RefreshCw,
  send: Send,
  settings: Settings,
  shield: Shield,
  spark: Sparkles,
  stop: Square,
  tool: Wrench,
};

export function Icon({ name, className = "" }: IconProps) {
  const Component = icons[name];
  return <Component aria-hidden="true" className={`icon ${className}`} strokeWidth={1.8} />;
}
