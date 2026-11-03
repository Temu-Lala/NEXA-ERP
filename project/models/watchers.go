package models

import (
	"context"
	"fmt"

	"nexa/addons/mail"
	"nexa/core/orm"
)

// Watchers on the agile work item. The thread rules live in the mail addon; this
// file says which of project's records take part (the model tags in models.go) and
// gives a person a way to put their own name on a record they care about, which is
// the one part of following a record that a tag cannot declare.

func init() {
	orm.RegisterCompute(taskModel, "follower_count", []string{"id"}, computeTaskFollowers)
	orm.RegisterCompute(taskModel, "is_followed", []string{"id"}, computeTaskFollowing)
	orm.RegisterObjectAction(taskModel, "action_follow", followTask)
	orm.RegisterObjectAction(taskModel, "action_unfollow", unfollowTask)
}

// computeTaskFollowers counts the people watching one issue. The count is a fact
// about a record the reader is already allowed to open, so it is read with
// elevated rights rather than filtered through the reader's own watch list.
func computeTaskFollowers(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return mail.FollowerCount(ctx, taskModel, intID(rec["id"])), nil
}

// computeTaskFollowing is session state: whether the reader is on this issue's
// thread. Without a session there is nobody to ask about, and no query is made.
func computeTaskFollowing(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	uid := orm.UIDFromContext(ctx)
	if uid <= 0 {
		return false, nil
	}
	return mail.Following(ctx, taskModel, intID(rec["id"]), uid), nil
}

// followTask subscribes the caller to the issue. Watching is something you do to
// your own inbox, so the action takes no target user and cannot be pointed at a
// colleague.
func followTask(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.UIDFromContext(ctx)
	if uid <= 0 {
		return "", fmt.Errorf("a session is required to follow a task")
	}
	if err := mail.Follow(ctx, taskModel, id, uid); err != nil {
		return "", err
	}
	return "You are following this task.", nil
}

// unfollowTask drops the caller's subscription. Not following is the same end
// state whether the caller was watching or not, so this never reports an error for
// a button pressed twice.
func unfollowTask(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.UIDFromContext(ctx)
	if uid <= 0 {
		return "", fmt.Errorf("a session is required to unfollow a task")
	}
	if err := mail.Unfollow(ctx, taskModel, id, uid); err != nil {
		return "", err
	}
	return "You no longer follow this task.", nil
}
