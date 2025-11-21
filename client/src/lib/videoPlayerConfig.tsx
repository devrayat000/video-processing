import type { IconSet, VideoContainerStyles } from '@ntxmjs/react-custom-video-player'
import {
  PlayIcon,
  PauseIcon,
  VolumeIcon,
  VolumeMuteIcon,
  FullscreenIcon,
  ExitFullscreenIcon,
  PipIcon,
  SettingsIcon,
  SpeedIcon,
  CCIcon,
  BackIcon,
  TheaterIcon,
  CheckIcon,
  HDChip,
  SleepIcon,
  StableVolumeIcon,
} from '@/components/VideoPlayerIcons'

export const playerIcons: IconSet = {
  play: <PlayIcon />,
  pause: <PauseIcon />,
  volume: <VolumeIcon />,
  volumeMute: <VolumeMuteIcon />,
  fullscreen: <FullscreenIcon />,
  exitFullscreen: <ExitFullscreenIcon />,
  pip: <PipIcon />,
  settings: <SettingsIcon />,
  speed: <SpeedIcon />,
  cc: <CCIcon />,
  back: <BackIcon />,
  theater: <TheaterIcon />,
  check: <CheckIcon />,
  hdChip: <HDChip />,
  sleepTimer: <SleepIcon />,
  stableVolume: <StableVolumeIcon />,
}

export const videoContainerStyles: VideoContainerStyles = {
  parent: {
    width: '100%',
    height: '100%',
  },
  video: {
    width: '100%',
    height: '100%',
  },
}
