declare module "@ntxmjs/react-custom-video-player" {
  import { FC, ReactNode, CSSProperties } from "react";

  export interface CaptionTrack {
    src: string;
    srclang: string;
    label: string;
    default?: boolean;
  }

  export interface IconSet {
    play: ReactNode;
    pause: ReactNode;
    volume: ReactNode;
    volumeMute: ReactNode;
    fullscreen: ReactNode;
    exitFullscreen: ReactNode;
    pip: ReactNode;
    settings: ReactNode;
    speed: ReactNode;
    cc: ReactNode;
    back: ReactNode;
    theater: ReactNode;
    check: ReactNode;
    hdChip: ReactNode;
    sleepTimer: ReactNode;
    stableVolume: ReactNode;
  }

  export interface VideoContainerStyles {
    parent: CSSProperties;
    video: CSSProperties;
  }

  export interface CustomVideoPlayerProps {
    src: string;
    poster: string;
    theme: "light" | "dark" | Theme;
    icons: IconSet;
    videoContainerStyles: VideoContainerStyles;
    captions?: CaptionTrack[];
    startTime?: number;
    stableVolume?: boolean;
    className?: string;
    controlSize?: number;
    type?: "hls" | "mp4" | "auto";
  }

  export interface Theme {
    colors: {
      background: string;
      controlsBottomBG: string;
      backgroundSecondary: string;
      backgroundTertiary: string;
      text: string;
      textMuted: string;
      accent: string;
      accentHover: string;
      line: string;
      controlBg: string;
      controlBgHover: string;
      controlBorder: string;
      panelBg: string;
      panelBorder: string;
      shadowColor: string;
      centerBigBtnStyle: CSSProperties & {
        bgOnMouseEnter?: string;
        borderOnMouseEnter?: string;
      };
      customCaptionStyle: CSSProperties;
    };
    fonts: {
      primary: string;
      size: {
        small: string;
        medium: string;
        large: string;
      };
    };
    borderRadius: {
      small: string;
      medium: string;
      large: string;
    };
    spacing: {
      small: string;
      medium: string;
      large: string;
    };
  }

  export const CustomVideoPlayer: FC<CustomVideoPlayerProps>;
}
