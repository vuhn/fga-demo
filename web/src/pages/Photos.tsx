import ContentPage, { type Kind } from "../components/ContentPage";

const photos: Kind = {
  title: "Photos",
  container: "album",
  item: "photo",
  containerRef: "album_id",
  createFeature: "album.create",
  createPrivateFeature: "album.createPrivate",
  defaultMime: "image/jpeg",
};

export default function Photos() {
  return <ContentPage kind={photos} />;
}
