# DreamBite

[![GitHub release (latest by date)](https://img.shields.io/github/v/release/ghostiam/DreamBite)](https://github.com/ghostiam/DreamBite/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[Русская версия](README_RU.md)

**DreamBite - now your avatar can actually bite!** \
Grab your friends by their ears, tails, or anything else that looks biteable.

This is an open-source tool for interacting with PhysBones that takes just a couple of minutes to set up:
drop the prefab, adjust the mouth position, and you're good to go! \
Works with face tracking and manual mode.

https://github.com/user-attachments/assets/66449eeb-d040-489c-89da-a026e0b159fa

## Download
Grab the latest version from the [GitHub Releases](https://github.com/ghostiam/DreamBite/releases) section.

## Requirements
- [VRChat SDK3](https://vrchat.com/home/download)
- [VRCFury](https://vrcfury.com)

## Setup
1. Import the `unitypackage` into your project.
2. Drag and drop the `DreamBite` prefab from `Assets/GhostIAm/DreamBite` onto your avatar.
3. Adjust the collider so it’s inside the mouth, with the arrow pointing outward. \
   ![head.png](Docs/head.jpg)
4. Make sure the collider arrow on your chosen hand points toward the palm (the direction your hand closes). If it doesn't, select `Custom` instead of `Auto` in the component settings and adjust it manually. \
   ![hand.png](Docs/hand.jpg)
5. Done!

## Using with Face Tracking
You'll need to run the `DreamBiteApp` external app (included in the archive).

**How to bite:**
1. Open your hand (Open Hand gesture).
2. Open your mouth for half a second (adjustable via the `DreamBiteDelay` animation length).
3. Close your mouth. If sound is enabled, you'll hear a "nom".

**How to let go:**
1. Just open your mouth.

## Manual Use (No Face Tracking)
1. Open your Radial Menu.
2. Select `DreamBite`.
3. Toggle `Manual enable`.
4. Toggle `Manual Move Collider`. This moves the grab area from your hand to your mouth.
> \[!NOTE\]
> You won't be able to grab things with that hand normally anymore, but squeezing your controller will trigger the bite!

## Contributing
Any help is welcome! Feel free to:
- [Report a bug](https://github.com/ghostiam/DreamBite/issues) or suggest a new feature.
- Create a [Pull Request](https://github.com/ghostiam/DreamBite/pulls) with your improvements.

## Usage & Rights
You are free to use this asset however you like, including on commercial avatars for sale.
- **Allowed:** Use on commercial avatars, modify the asset, and include it in any of your projects.
- **Prohibited:** Claiming this asset or its parts as your own work.

Credit is not required but is always greatly appreciated!

## License
This project is licensed under the [MIT License](LICENSE).