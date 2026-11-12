import ContentPage, { type Kind } from "../components/ContentPage";
import type { Project } from "../api";

const photos: Kind = {
  title: "Photos",
  container: "album",
  item: "photo",
  containerRef: "album_id",
  createAction: "createAlbum",
  createPrivateAction: "createPrivateAlbum",
  defaultMime: "image/jpeg",
};

export default function Photos({ project }: { project: Project }) {
  return <ContentPage kind={photos} project={project} />;
}
