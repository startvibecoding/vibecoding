# VibeCoding Assets

This directory contains branding assets for VibeCoding.

## Files

| File | Description | Usage |
|------|-------------|-------|
| `icon.svg` | Full icon (512×512) | Deb packaging, documentation hero |
| `logo.svg` | Compact logo (128×128) | README, favicons, small displays |

## Icon Design

The VibeCoding icon represents:

- **Terminal Window**: The core interface of the tool
- **Prompt Symbol (❯)**: Command-line interaction
- **AI Brain**: Neural network symbolizing AI capabilities
- **Color Scheme**: 
  - Cyan (#00d4ff) - Primary accent, represents technology
  - Purple (#7b2ff7) - AI and intelligence
  - Coral (#ff6b6b) - Energy and creativity
  - Dark (#1a1a2e) - Terminal aesthetic

## Usage in Deb Packaging

For `.deb` packages, convert the SVG to PNG:

```bash
# Install dependencies
sudo apt install librsvg2-bin

# Convert to PNG (512×512)
rsvg-convert -w 512 -h 512 assets/icon.svg -o assets/icon.png

# Convert to smaller sizes for different contexts
rsvg-convert -w 256 -h 256 assets/icon.svg -o assets/icon-256.png
rsvg-convert -w 128 -h 128 assets/icon.svg -o assets/icon-128.png
rsvg-convert -w 64 -h 64 assets/icon.svg -o assets/icon-64.png
rsvg-convert -w 48 -h 48 assets/icon.svg -o assets/icon-48.png
```

Place the PNG files in the deb package structure:

```
debian/
├── usr/
│   └── share/
│       └── icons/
│           └── hicolor/
│               ├── 48x48/
│               │   └── apps/
│               │       └── vibecoding.png
│               ├── 128x128/
│               │   └── apps/
│               │       └── vibecoding.png
│               └── 256x256/
│                   └── apps/
│                       └── vibecoding.png
└── usr/
    └── share/
        └── applications/
            └── vibecoding.desktop
```

## Desktop Entry

Example `vibecoding.desktop`:

```desktop
[Desktop Entry]
Name=VibeCoding
Comment=AI Coding Assistant
Exec=vibecoding
Icon=vibecoding
Terminal=true
Type=Application
Categories=Development;
```

## License

These assets are part of the VibeCoding project and follow the same MIT license.
