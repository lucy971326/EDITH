type IconName = "attachment" | "chevron" | "close" | "file" | "folder" | "grid" | "plus" | "refresh" | "send" | "settings" | "shield" | "spark" | "stop" | "tool";

type IconProps = { name: IconName; className?: string };

export function Icon({ name, className = "" }: IconProps) {
  const paths: Record<IconName, React.ReactNode> = {
    attachment: <path d="m8.5 12.5 6.7-6.7a3.2 3.2 0 1 1 4.6 4.6l-8.6 8.6a5 5 0 0 1-7-7l8-8" />,
    chevron: <path d="m7 9 5 5 5-5" />,
    close: <path d="m6 6 12 12M18 6 6 18" />,
    file: <path d="M6 3h8l4 4v14H6zM14 3v5h5" />,
    folder: <path d="M3 7a2 2 0 0 1 2-2h5l2 2h7a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />,
    grid: <><rect x="4" y="4" width="6" height="6" /><rect x="14" y="4" width="6" height="6" /><rect x="4" y="14" width="6" height="6" /><rect x="14" y="14" width="6" height="6" /></>,
    plus: <path d="M12 5v14M5 12h14" />,
    refresh: <path d="M20 12a8 8 0 1 1-2.3-5.7M20 4v5h-5" />,
    send: <path d="m3 4 18 8-18 8 2.5-7H14l-10.5-2z" />,
    settings: <><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1-2.1 2.1-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.5v.2h-3v-.2a1.7 1.7 0 0 0-1-1.5 1.7 1.7 0 0 0-1.9.3l-.1.1-2.1-2.1.1-.1A1.7 1.7 0 0 0 7 15a1.7 1.7 0 0 0-1.5-1H5.3v-3h.2A1.7 1.7 0 0 0 7 10a1.7 1.7 0 0 0-.3-1.9l-.1-.1 2.1-2.1.1.1a1.7 1.7 0 0 0 1.9.3 1.7 1.7 0 0 0 1-1.5v-.2h3v.2a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.9-.3l.1-.1 2.1 2.1-.1.1a1.7 1.7 0 0 0-.3 1.9 1.7 1.7 0 0 0 1.5 1h.2v3h-.2a1.7 1.7 0 0 0-1.4 1z" /></>,
    shield: <path d="M12 3 20 6v5c0 5-3.4 8.7-8 10-4.6-1.3-8-5-8-10V6z" />,
    spark: <path d="m12 3 1.4 5.6L19 10l-5.6 1.4L12 17l-1.4-5.6L5 10l5.6-1.4z" />,
    stop: <rect x="6" y="6" width="12" height="12" rx="1" />,
    tool: <path d="M14.8 5.2a5 5 0 0 0-6.3 6.3l-5.2 5.2a2 2 0 0 0 2.8 2.8l5.2-5.2a5 5 0 0 0 6.3-6.3l-3 3-2.8-.8-.8-2.8z" />,
  };
  return <svg aria-hidden="true" className={`icon ${className}`} viewBox="0 0 24 24">{paths[name]}</svg>;
}
