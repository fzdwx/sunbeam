from typing import TypedDict
import pathlib
import argparse
import uuid
import json
from datetime import datetime


class JournalEntry(TypedDict):
    title: str
    content: str
    timestamp: int


class Journal(TypedDict):
    entries: dict[str, JournalEntry]


dirname = pathlib.Path(__file__).parent.absolute()


def load_journal() -> Journal:
    with open(dirname / "journal.json") as f:
        journal: Journal = json.load(f)

    return journal


def save_journal(journal: Journal) -> None:
    with open(dirname / "journal.json", "w") as f:
        json.dump(journal, f, indent=4)


def list_entries(journal: Journal):
    return {
        "type": "list",
        "showPreview": True,
        "items": [
            {
                "title": entry["title"],
                "preview": entry["content"],
                "accessories": [
                    datetime.utcfromtimestamp(entry["timestamp"]).strftime(
                        "%Y-%m-%d %H:%M:%S"
                    )
                ],
                "actions": [
                    {
                        "type": "copy-text",
                        "text": entry["content"],
                        "title": "Copy Message",
                    },
                    {
                        "type": "run-command",
                        "title": "Edit Journal",
                        "command": "edit-journal",
                        "onSuccess": "reload-page",
                    },
                    {
                        "type": "run-command",
                        "title": "New Entry",
                        "command": "new-entry",
                        "onSuccess": "reload-page",
                        "shortcut": "ctrl+n",
                        "with": {
                            "title": {"type": "textfield", "title": "Title"},
                            "content": {"type": "textarea", "title": "Content"},
                        },
                    },
                    {
                        "type": "run-command",
                        "title": "Delete Entry",
                        "command": "delete-entry",
                        "onSuccess": "reload-page",
                        "shortcut": "ctrl+d",
                        "with": {"uuid": uuid},
                    },
                    {
                        "type": "run-command",
                        "title": "Edit Entry",
                        "command": "edit-entry",
                        "shortcut": "ctrl+e",
                        "onSuccess": "reload-page",
                        "with": {
                            "uuid": uuid,
                            "title": {
                                "type": "textfield",
                                "title": "Title",
                                "default": entry["title"],
                            },
                            "content": {
                                "type": "textarea",
                                "title": "Content",
                                "default": entry["content"],
                            },
                        },
                    },
                ],
            }
            for uuid, entry in journal["entries"].items()
        ],
    }


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    commands = parser.add_subparsers(dest="command")
    commands.add_parser("list")

    edit_parser = commands.add_parser("edit")
    edit_parser.add_argument("--uuid", required=True)
    edit_parser.add_argument("--title", required=True)
    edit_parser.add_argument("--content", required=True)

    delete_parser = commands.add_parser("delete")
    delete_parser.add_argument("--uuid", required=True)

    new_parser = commands.add_parser("new")
    new_parser.add_argument("--title", required=True)
    new_parser.add_argument("--content", required=True)

    args = parser.parse_args()

    journal = load_journal()
    match args.command:
        case "list":
            print(json.dumps(list_entries(journal)))
        case "edit":
            journal["entries"][args.uuid] = {
                "title": args.title,
                "content": args.content,
                "timestamp": int(datetime.now().timestamp()),
            }
            save_journal(journal)
        case "delete":
            del journal["entries"][args.uuid]
            save_journal(journal)
        case "new":
            journal["entries"][str(uuid.uuid4())] = {
                "title": args.title,
                "content": args.content,
                "timestamp": int(datetime.now().timestamp()),
            }
            save_journal(journal)
