// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/validation"

	"xorm.io/builder"
)

// ErrEmailNotActivated e-mail address has not been activated error
var ErrEmailNotActivated = util.NewInvalidArgumentErrorf("e-mail address has not been activated")

// ErrEmailCharIsNotSupported e-mail address contains unsupported character
type ErrEmailCharIsNotSupported struct {
	Email string
}

// IsErrEmailCharIsNotSupported checks if an error is an ErrEmailCharIsNotSupported
func IsErrEmailCharIsNotSupported(err error) bool {
	_, ok := err.(ErrEmailCharIsNotSupported)
	return ok
}

func (err ErrEmailCharIsNotSupported) Error() string {
	return fmt.Sprintf("e-mail address contains unsupported character [email: %s]", err.Email)
}

func (err ErrEmailCharIsNotSupported) Unwrap() error {
	return util.ErrInvalidArgument
}

// ErrEmailInvalid represents an error where the email address does not comply with RFC 5322
// or has a leading '-' character
type ErrEmailInvalid struct {
	Email string
}

// IsErrEmailInvalid checks if an error is an ErrEmailInvalid
func IsErrEmailInvalid(err error) bool {
	_, ok := err.(ErrEmailInvalid)
	return ok
}

func (err ErrEmailInvalid) Error() string {
	return fmt.Sprintf("e-mail invalid [email: %s]", err.Email)
}

func (err ErrEmailInvalid) Unwrap() error {
	return util.ErrInvalidArgument
}

// ErrEmailAlreadyUsed represents a "EmailAlreadyUsed" kind of error.
type ErrEmailAlreadyUsed struct {
	Email string
}

// IsErrEmailAlreadyUsed checks if an error is a ErrEmailAlreadyUsed.
func IsErrEmailAlreadyUsed(err error) bool {
	_, ok := err.(ErrEmailAlreadyUsed)
	return ok
}

func (err ErrEmailAlreadyUsed) Error() string {
	return fmt.Sprintf("e-mail already in use [email: %s]", err.Email)
}

func (err ErrEmailAlreadyUsed) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrEmailAddressNotExist email address not exist
type ErrEmailAddressNotExist struct {
	Email string
}

// IsErrEmailAddressNotExist checks if an error is an ErrEmailAddressNotExist
func IsErrEmailAddressNotExist(err error) bool {
	_, ok := err.(ErrEmailAddressNotExist)
	return ok
}

func (err ErrEmailAddressNotExist) Error() string {
	return fmt.Sprintf("Email address does not exist [email: %s]", err.Email)
}

func (err ErrEmailAddressNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrPrimaryEmailCannotDelete primary email address cannot be deleted
type ErrPrimaryEmailCannotDelete struct {
	Email string
}

// IsErrPrimaryEmailCannotDelete checks if an error is an ErrPrimaryEmailCannotDelete
func IsErrPrimaryEmailCannotDelete(err error) bool {
	_, ok := err.(ErrPrimaryEmailCannotDelete)
	return ok
}

func (err ErrPrimaryEmailCannotDelete) Error() string {
	return fmt.Sprintf("Primary email address cannot be deleted [email: %s]", err.Email)
}

func (err ErrPrimaryEmailCannotDelete) Unwrap() error {
	return util.ErrInvalidArgument
}

// EmailAddress is the list of all email addresses of a user. It also contains the
// primary email address which is saved in user table.
type EmailAddress struct {
	ID          int64  `xorm:"pk autoincr"`
	UID         int64  `xorm:"INDEX NOT NULL"`
	Email       string `xorm:"UNIQUE NOT NULL"`
	LowerEmail  string `xorm:"UNIQUE NOT NULL"`
	IsActivated bool
	IsPrimary   bool `xorm:"DEFAULT(false) NOT NULL"`
}

func init() {
	db.RegisterModel(new(EmailAddress))
}

// BeforeInsert will be invoked by XORM before inserting a record
func (email *EmailAddress) BeforeInsert() {
	if email.LowerEmail == "" {
		email.LowerEmail = strings.ToLower(email.Email)
	}
}

func InsertEmailAddress(ctx context.Context, email *EmailAddress) (*EmailAddress, error) {
	if err := db.Insert(ctx, email); err != nil {
		return nil, err
	}
	return email, nil
}

func UpdateEmailAddress(ctx context.Context, email *EmailAddress) error {
	_, err := db.GetEngine(ctx).ID(email.ID).AllCols().Update(email)
	return err
}

var emailRegexp = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+-/=?^_`{|}~]*@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

// ValidateEmail check if email is a allowed address
func ValidateEmail(email string) error {
	if len(email) == 0 {
		return ErrEmailInvalid{email}
	}

	if !emailRegexp.MatchString(email) {
		return ErrEmailCharIsNotSupported{email}
	}

	if email[0] == '-' {
		return ErrEmailInvalid{email}
	}

	if _, err := mail.ParseAddress(email); err != nil {
		return ErrEmailInvalid{email}
	}

	// if there is no allow list, then check email against block list
	if len(setting.Service.EmailDomainAllowList) == 0 &&
		validation.IsEmailDomainListed(setting.Service.EmailDomainBlockList, email) {
		return ErrEmailInvalid{email}
	}

	// if there is an allow list, then check email against allow list
	if len(setting.Service.EmailDomainAllowList) > 0 &&
		!validation.IsEmailDomainListed(setting.Service.EmailDomainAllowList, email) {
		return ErrEmailInvalid{email}
	}

	return nil
}

func GetEmailAddressByEmail(ctx context.Context, email string) (*EmailAddress, error) {
	ea := &EmailAddress{}
	if has, err := db.GetEngine(ctx).Where("lower_email=?", strings.ToLower(email)).Get(ea); err != nil {
		return nil, err
	} else if !has {
		return nil, ErrEmailAddressNotExist{email}
	}
	return ea, nil
}

func GetEmailAddressOfUser(ctx context.Context, email string, uid int64) (*EmailAddress, error) {
	ea := &EmailAddress{}
	if has, err := db.GetEngine(ctx).Where("lower_email=? AND uid=?", strings.ToLower(email), uid).Get(ea); err != nil {
		return nil, err
	} else if !has {
		return nil, ErrEmailAddressNotExist{email}
	}
	return ea, nil
}

func GetPrimaryEmailAddressOfUser(ctx context.Context, uid int64) (*EmailAddress, error) {
	ea := &EmailAddress{}
	if has, err := db.GetEngine(ctx).Where("uid=? AND is_primary=?", uid, true).Get(ea); err != nil {
		return nil, err
	} else if !has {
		return nil, ErrEmailAddressNotExist{}
	}
	return ea, nil
}

// GetEmailAddresses returns all email addresses belongs to given user.
func GetEmailAddresses(ctx context.Context, uid int64) ([]*EmailAddress, error) {
	emails := make([]*EmailAddress, 0, 5)
	if err := db.GetEngine(ctx).
		Where("uid=?", uid).
		Asc("id").
		Find(&emails); err != nil {
		return nil, err
	}
	return emails, nil
}

// GetEmailAddressByID gets a user's email address by ID
func GetEmailAddressByID(ctx context.Context, uid, id int64) (*EmailAddress, error) {
	// User ID is required for security reasons
	email := &EmailAddress{UID: uid}
	if has, err := db.GetEngine(ctx).ID(id).Get(email); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	return email, nil
}

// IsEmailActive check if email is activated with a different emailID
func IsEmailActive(ctx context.Context, email string, excludeEmailID int64) (bool, error) {
	if len(email) == 0 {
		return true, nil
	}

	// Can't filter by boolean field unless it's explicit
	cond := builder.NewCond()
	cond = cond.And(builder.Eq{"lower_email": strings.ToLower(email)}, builder.Neq{"id": excludeEmailID})
	if setting.Service.RegisterEmailConfirm {
		// Inactive (unvalidated) addresses don't count as active if email validation is required
		cond = cond.And(builder.Eq{"is_activated": true})
	}

	var em EmailAddress
	if has, err := db.GetEngine(ctx).Where(cond).Get(&em); has || err != nil {
		if has {
			log.Info("isEmailActive(%q, %d) found duplicate in email ID %d", email, excludeEmailID, em.ID)
		}
		return has, err
	}

	return false, nil
}

// IsEmailUsed returns true if the email has been used.
func IsEmailUsed(ctx context.Context, email string) (bool, error) {
	if len(email) == 0 {
		return true, nil
	}

	return db.GetEngine(ctx).Where("lower_email=?", strings.ToLower(email)).Get(&EmailAddress{})
}

// DeleteInactiveEmailAddresses deletes inactive email addresses
func DeleteInactiveEmailAddresses(ctx context.Context) error {
	_, err := db.GetEngine(ctx).
		Where("is_activated = ?", false).
		Delete(new(EmailAddress))
	return err
}

// ActivateEmail activates the email address to given user.
func ActivateEmail(ctx context.Context, email *EmailAddress) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()
	if err := updateActivation(ctx, email, true); err != nil {
		return err
	}
	return committer.Commit()
}

func updateActivation(ctx context.Context, email *EmailAddress, activate bool) error {
	user, err := GetUserByID(ctx, email.UID)
	if err != nil {
		return err
	}
	if user.Rands, err = GetUserSalt(); err != nil {
		return err
	}
	email.IsActivated = activate
	if _, err := db.GetEngine(ctx).ID(email.ID).Cols("is_activated").Update(email); err != nil {
		return err
	}
	return UpdateUserCols(ctx, user, "rands")
}

// MakeEmailPrimary sets primary email address of given user.
func MakeEmailPrimary(ctx context.Context, email *EmailAddress) error {
	has, err := db.GetEngine(ctx).Get(email)
	if err != nil {
		return err
	} else if !has {
		return ErrEmailAddressNotExist{Email: email.Email}
	}

	if !email.IsActivated {
		return ErrEmailNotActivated
	}

	user := &User{}
	has, err = db.GetEngine(ctx).ID(email.UID).Get(user)
	if err != nil {
		return err
	} else if !has {
		return ErrUserNotExist{
			UID: email.UID,
		}
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	// 1. Update user table
	user.Email = email.Email
	if _, err = sess.ID(user.ID).Cols("email").Update(user); err != nil {
		return err
	}

	// 2. Update old primary email
	if _, err = sess.Where("uid=? AND is_primary=?", email.UID, true).Cols("is_primary").Update(&EmailAddress{
		IsPrimary: false,
	}); err != nil {
		return err
	}

	// 3. update new primary email
	email.IsPrimary = true
	if _, err = sess.ID(email.ID).Cols("is_primary").Update(email); err != nil {
		return err
	}

	return committer.Commit()
}

// VerifyActiveEmailCode verifies active email code when active account
func VerifyActiveEmailCode(ctx context.Context, code, email string) *EmailAddress {
	minutes := setting.Service.ActiveCodeLives

	if user := GetVerifyUser(ctx, code); user != nil {
		// time limit code
		prefix := code[:base.TimeLimitCodeLength]
		data := fmt.Sprintf("%d%s%s%s%s", user.ID, email, user.LowerName, user.Passwd, user.Rands)

		if base.VerifyTimeLimitCode(data, minutes, prefix) {
			emailAddress := &EmailAddress{UID: user.ID, Email: email}
			if has, _ := db.GetEngine(ctx).Get(emailAddress); has {
				return emailAddress
			}
		}
	}
	return nil
}

// SearchEmailOrderBy is used to sort the results from SearchEmails()
type SearchEmailOrderBy string

func (s SearchEmailOrderBy) String() string {
	return string(s)
}

// Strings for sorting result
const (
	SearchEmailOrderByEmail        SearchEmailOrderBy = "email_address.lower_email ASC, email_address.is_primary DESC, email_address.id ASC"
	SearchEmailOrderByEmailReverse SearchEmailOrderBy = "email_address.lower_email DESC, email_address.is_primary ASC, email_address.id DESC"
	SearchEmailOrderByName         SearchEmailOrderBy = "`user`.lower_name ASC, email_address.is_primary DESC, email_address.id ASC"
	SearchEmailOrderByNameReverse  SearchEmailOrderBy = "`user`.lower_name DESC, email_address.is_primary ASC, email_address.id DESC"
)

// SearchEmailOptions are options to search e-mail addresses for the admin panel
type SearchEmailOptions struct {
	db.ListOptions
	Keyword     string
	SortType    SearchEmailOrderBy
	IsPrimary   util.OptionalBool
	IsActivated util.OptionalBool
}

// SearchEmailResult is an e-mail address found in the user or email_address table
type SearchEmailResult struct {
	UID         int64
	Email       string
	IsActivated bool
	IsPrimary   bool
	// From User
	Name     string
	FullName string
}

// SearchEmails takes options i.e. keyword and part of email name to search,
// it returns results in given range and number of total results.
func SearchEmails(ctx context.Context, opts *SearchEmailOptions) ([]*SearchEmailResult, int64, error) {
	var cond builder.Cond = builder.Eq{"`user`.`type`": UserTypeIndividual}
	if len(opts.Keyword) > 0 {
		likeStr := "%" + strings.ToLower(opts.Keyword) + "%"
		cond = cond.And(builder.Or(
			builder.Like{"lower(`user`.full_name)", likeStr},
			builder.Like{"`user`.lower_name", likeStr},
			builder.Like{"email_address.lower_email", likeStr},
		))
	}

	switch {
	case opts.IsPrimary.IsTrue():
		cond = cond.And(builder.Eq{"email_address.is_primary": true})
	case opts.IsPrimary.IsFalse():
		cond = cond.And(builder.Eq{"email_address.is_primary": false})
	}

	switch {
	case opts.IsActivated.IsTrue():
		cond = cond.And(builder.Eq{"email_address.is_activated": true})
	case opts.IsActivated.IsFalse():
		cond = cond.And(builder.Eq{"email_address.is_activated": false})
	}

	count, err := db.GetEngine(ctx).Join("INNER", "`user`", "`user`.ID = email_address.uid").
		Where(cond).Count(new(EmailAddress))
	if err != nil {
		return nil, 0, fmt.Errorf("Count: %w", err)
	}

	orderby := opts.SortType.String()
	if orderby == "" {
		orderby = SearchEmailOrderByEmail.String()
	}

	opts.SetDefaultValues()

	emails := make([]*SearchEmailResult, 0, opts.PageSize)
	err = db.GetEngine(ctx).Table("email_address").
		Select("email_address.*, `user`.name, `user`.full_name").
		Join("INNER", "`user`", "`user`.ID = email_address.uid").
		Where(cond).
		OrderBy(orderby).
		Limit(opts.PageSize, (opts.Page-1)*opts.PageSize).
		Find(&emails)

	return emails, count, err
}

// ActivateUserEmail will change the activated state of an email address,
// either primary or secondary (all in the email_address table)
func ActivateUserEmail(ctx context.Context, userID int64, email string, activate bool) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	// Activate/deactivate a user's secondary email address
	// First check if there's another user active with the same address
	addr, exist, err := db.Get[EmailAddress](ctx, builder.Eq{"uid": userID, "lower_email": strings.ToLower(email)})
	if err != nil {
		return err
	} else if !exist {
		return fmt.Errorf("no such email: %d (%s)", userID, email)
	}

	if addr.IsActivated == activate {
		// Already in the desired state; no action
		return nil
	}
	if activate {
		if used, err := IsEmailActive(ctx, email, addr.ID); err != nil {
			return fmt.Errorf("unable to check isEmailActive() for %s: %w", email, err)
		} else if used {
			return ErrEmailAlreadyUsed{Email: email}
		}
	}
	if err = updateActivation(ctx, addr, activate); err != nil {
		return fmt.Errorf("unable to updateActivation() for %d:%s: %w", addr.ID, addr.Email, err)
	}

	// Activate/deactivate a user's primary email address and account
	if addr.IsPrimary {
		user, exist, err := db.Get[User](ctx, builder.Eq{"id": userID, "email": email})
		if err != nil {
			return err
		} else if !exist {
			return fmt.Errorf("no user with ID: %d and Email: %s", userID, email)
		}

		// The user's activation state should be synchronized with the primary email
		if user.IsActive != activate {
			user.IsActive = activate
			if user.Rands, err = GetUserSalt(); err != nil {
				return fmt.Errorf("unable to generate salt: %w", err)
			}
			if err = UpdateUserCols(ctx, user, "is_active", "rands"); err != nil {
				return fmt.Errorf("unable to updateUserCols() for user ID: %d: %w", userID, err)
			}
		}
	}

	return committer.Commit()
}
