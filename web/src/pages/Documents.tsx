import ContentPage, { type Kind } from "../components/ContentPage";
import type { Project } from "../api";

const documents: Kind = {
  title: "Documents",
  container: "folder",
  item: "document",
  containerRef: "folder_id",
  createAction: "createFolder",
  createPrivateAction: "createPrivateFolder",
  defaultMime: "text/plain",
};

export default function Documents({ project }: { project: Project }) {
  return <ContentPage kind={documents} project={project} />;
}
