import type {
  IconSet,
  VideoContainerStyles,
} from "@ntxmjs/react-custom-video-player";
import {
  // VolumeMuteIcon,
  // FullscreenIcon,
  // ExitFullscreenIcon,
  // PipIcon,
  // SettingsIcon,
  // SpeedIcon,
  // CCIcon,
  // BackIcon,
  // TheaterIcon,
  // CheckIcon,
  HDChip,
  // SleepIcon,
  // StableVolumeIcon,
} from "@/components/VideoPlayerIcons";
import {
  AudioLinesIcon,
  CheckIcon,
  ChevronLeftIcon,
  ClosedCaptionIcon,
  FullscreenIcon,
  GaugeIcon,
  HourglassIcon,
  MinimizeIcon,
  PauseIcon,
  PictureInPicture2Icon,
  PlayIcon,
  SettingsIcon,
  FilmIcon,
  Volume1Icon,
  VolumeXIcon,
} from "lucide-react";

export const playerIcons: IconSet = {
  play: <PlayIcon />,
  pause: <PauseIcon />,
  volume: <Volume1Icon />,
  volumeMute: <VolumeXIcon />,
  fullscreen: <FullscreenIcon />,
  exitFullscreen: <MinimizeIcon />,
  pip: <PictureInPicture2Icon />,
  settings: <SettingsIcon />,
  speed: <GaugeIcon />,
  cc: <ClosedCaptionIcon />,
  back: <ChevronLeftIcon />,
  theater: <FilmIcon />,
  check: <CheckIcon />,
  hdChip: <HDChip />,
  sleepTimer: <HourglassIcon />,
  stableVolume: <AudioLinesIcon />,
};

export const videoContainerStyles: VideoContainerStyles = {
  parent: {
    width: "100%",
    height: "100%",
  },
  video: {
    width: "100%",
    height: "100%",
  },
};
