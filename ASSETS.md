# Game Artwork

The maintainer confirmed on 2026-11-24 that the 28 character images were AI-generated. Character names, IDs and image paths are listed in `assets/PROVENANCE.json`. The generation tool and its terms have not been supplied.

Character artwork now lives alongside its definitions under `content/cards/<id>/assets/`; normal enemies use `content/enemies/` and bosses use `content/bosses/`. Shared scenes and UI artwork remain under `assets/`. See [content maintenance](content/README.md). Image bytes were preserved during the move; legacy report paths are resolved through the generated unit asset aliases.

The maintainer requested publication of the existing card game, including the supplied artwork. The artwork is retained for inspection, but no independent public artwork license has been specified and the generation tool's terms have not been verified. Public availability is not a commercial-use or redistribution license. Before reusing or redistributing an image, confirm its permitted uses, including modification and commercial use. Unconfirmed images can instead be replaced with distributable placeholders.

Background and UI artwork provenance still needs confirmation. The code's AGPL license does not automatically establish an artwork license.

The previous asset inspection found no EXIF or XMP metadata in the 95 WebP files; animation/playback metadata is not a licensing review. The source ZIP is checked again for known private markers, including binary metadata.
