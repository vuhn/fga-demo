import ContentPage, { type Kind } from "../components/ContentPage";

const documents: Kind = {
  title: "Documents",
  container: "folder",
  item: "document",
  containerRef: "folder_id",
  createFeature: "folder.create",
  createPrivateFeature: "folder.createPrivate",
  defaultMime: "text/plain",
};

export default function Documents() {
  return <ContentPage kind={documents} />;
}
