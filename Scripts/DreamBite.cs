#if UNITY_EDITOR

using System;
using System.Collections;
using System.Linq;
using UnityEditor;
using UnityEngine;
using VRC.Dynamics;
using VRC.SDK3.Avatars.Components;
using VRC.SDK3.Dynamics.Constraint.Components;
using VRC.SDKBase;
using VRC.SDKBase.Editor.BuildPipeline;

namespace GhostIAm.DreamBite
{
    [AddComponentMenu("GhostIAm/DreamBite")]
    public class DreamBite : MonoBehaviour, IEditorOnly
    {
        public Hand hand = Hand.Right;
        public FixHandRotationMode fixHandRotationMode = FixHandRotationMode.Auto;
        public Quaternion fixHandRotation = Quaternion.identity;

        public Color gizmoColor = new(1, 0.5f, 0);
        public Transform handProxy;
        public Transform mouthTarget;

        private Vector3 _cachedPosition;
        private Quaternion _cachedRotation;

        public enum Hand
        {
            Left,
            Right
        }

        public enum FixHandRotationMode
        {
            None,
            Auto,
            Custom
        }

        private void OnDrawGizmosSelected()
        {
            if (_cachedPosition == transform.position && _cachedRotation == transform.rotation)
                return;

            UpdateSettings();
        }

        private void UpdateSettings()
        {
            _cachedPosition = transform.position;
            _cachedRotation = transform.rotation;

            var avatar = GetAvatar();
            if (!avatar) return;

            var animator = avatar.GetComponent<Animator>();
            if (!animator) return;

            var isRightHand = hand == Hand.Right;
            var handBone = isRightHand ? HumanBodyBones.RightHand : HumanBodyBones.LeftHand;
            var handTransform = animator.GetBoneTransform(handBone);

            if (!handTransform)
            {
                Log(LogType.Warning, $"Bone {handBone} not found on animator.");
                return;
            }

            var scale = VRCAvatarDescriptor.MaxScale(handTransform.lossyScale);

            var handCollider = isRightHand ? avatar.collider_handR : avatar.collider_handL;
            var handColliderHeight = handCollider.height * scale;
            var handColliderRadius = handCollider.radius * scale;

            var handColliderPos = handTransform.TransformPoint(handCollider.position);
            var handColliderRot = handTransform.rotation * handCollider.rotation;

            SetupHandProxy(handColliderPos, handColliderRot, handColliderRadius, handColliderHeight);
            SetupMouthTarget(handColliderRadius, handColliderHeight);
        }

        private VRCAvatarDescriptor GetAvatar()
        {
            return GetComponentInParent<VRCAvatarDescriptor>();
        }

        private void SetupHandProxy(Vector3 handColliderPos, Quaternion handColliderRot, float handColliderRadius,
            float handColliderHeight)
        {
            // Check hand proxy exists.
            if (!handProxy)
            {
                Log(LogType.Error, "No hand proxy found.");
                return;
            }

            // Set hand proxy transform.

            var rotation = fixHandRotationMode switch
            {
                FixHandRotationMode.None => handColliderRot,
                FixHandRotationMode.Auto => CalculateAutoRotation(),
                FixHandRotationMode.Custom => handColliderRot * fixHandRotation,
                _ => Quaternion.identity
            };

            handProxy.transform.position = handColliderPos;
            handProxy.transform.rotation = rotation;
            handProxy.transform.localScale = Vector3.one;

            // VRCParentConstraint

            ConfigureHandProxyConstraint();
            SetupGizmos(handProxy, handColliderRadius, handColliderHeight);

            UpdateHandInVrcFuryArmatureLink();
        }

        private Quaternion CalculateAutoRotation()
        {
            var avatar = GetAvatar();
            if (!avatar) return Quaternion.identity;

            var downInAvatarSpace = avatar.transform.up * -1;
            var targetRotation = Quaternion.LookRotation(downInAvatarSpace, -avatar.transform.forward);
            return targetRotation;
        }

        private void ConfigureHandProxyConstraint()
        {
            var constraint = handProxy.GetComponent<VRCParentConstraint>();
            if (!constraint)
            {
                throw new Exception("VRCParentConstraint component not found on hand proxy.");
            }

            constraint.IsActive = true;
            constraint.GlobalWeight = 0f;

            constraint.Locked = true;
            constraint.PositionAtRest = handProxy.transform.localPosition;
            constraint.RotationAtRest = handProxy.transform.localRotation.eulerAngles;
            constraint.AffectsPositionX = constraint.AffectsPositionY = constraint.AffectsPositionZ = true;
            constraint.AffectsRotationX = constraint.AffectsRotationY = constraint.AffectsRotationZ = true;

            var source = constraint.Sources.Count == 1
                ? constraint.Sources[0]
                : VRCConstraintSource.CreateDefault();

            source.SourceTransform = mouthTarget;
            source.Weight = 1f;
            source.ParentPositionOffset = Vector3.zero;
            source.ParentRotationOffset = Vector3.zero;

            if (constraint.Sources.Count != 1)
            {
                constraint.Sources.Clear();
                constraint.Sources.Add(source);
            }
            else
            {
                constraint.Sources[0] = source;
            }
        }

        private void SetupGizmos(Transform target, float radius, float height)
        {
            var gizmos = target.GetComponent<DreamBiteGizmos>();
            if (!gizmos)
            {
                throw new Exception("DreamBiteGizmos component not found on hand proxy.");
            }

            gizmos.color = gizmoColor;
            gizmos.preservingSize = false;
            gizmos.drawCapsule = true;
            gizmos.radius = radius;
            gizmos.length = height;
            gizmos.drawArrow = true;
            gizmos.arrowStart = Vector3.zero;
            gizmos.arrowEnd = Vector3.forward * (radius * 4f);
            gizmos.arrowCapSize = radius;
            gizmos.position = Vector3.zero;
            gizmos.rotation = Quaternion.identity;
        }

        private void UpdateHandInVrcFuryArmatureLink()
        {
            var component = handProxy.GetComponents<MonoBehaviour>()
                .FirstOrDefault(c => c != null && c.GetType().Assembly.GetName().Name == "VRCFury");
            if (!component)
            {
                throw new Exception(
                    "VRCFury component not found on hand proxy.");
            }

            var vrcf = component.GetType().GetField("content");
            if (vrcf == null)
                throw new Exception(
                    "Unable to access 'content' field in VRCFury component. VRCFury API may have changed.");

            var content = vrcf.GetValue(component);
            if (content == null)
                throw new Exception("VRCFury component 'content' field is null.");

            var armatureLinkType = content.GetType();
            if (armatureLinkType.Name != "ArmatureLink")
                throw new Exception(
                    $"Expected VRCFury content type 'ArmatureLink', but found '{armatureLinkType.Name}'.");

            var linkToField = armatureLinkType.GetField("linkTo");
            if (linkToField == null)
                throw new Exception("Unable to access 'linkTo' field in ArmatureLink.");

            var linkToList = linkToField.GetValue(content) as IList;
            if (linkToList == null)
                throw new Exception(
                    "ArmatureLink 'linkTo' field is not a valid list.");

            var count = linkToList.Count;
            if (count == 0)
                throw new Exception(
                    "ArmatureLink 'linkTo' list is empty. VRCFury API may have changed.");

            var firstElement = linkToList[0];

            var boneField = firstElement.GetType().GetField("bone");
            if (boneField == null)
                throw new Exception("Unable to access 'bone' field in LinkTo element. VRCFury API may have changed.");

            var isRightHand = hand == Hand.Right;
            var handBone = isRightHand ? HumanBodyBones.RightHand : HumanBodyBones.LeftHand;

            var currentBone = boneField.GetValue(firstElement) as HumanBodyBones?;
            if (currentBone == handBone) return;

            boneField.SetValue(firstElement, handBone);

            EditorUtility.SetDirty(component);
        }

        private void SetupMouthTarget(float handColliderRadius, float handColliderHeight)
        {
            // Check mouth target exists.
            if (!mouthTarget)
            {
                Log(LogType.Error, "No mouth target found.");
                return;
            }

            // Set transform.

            mouthTarget.transform.position = transform.position;
            mouthTarget.transform.rotation = transform.rotation * Quaternion.AngleAxis(90, Vector3.forward);
            mouthTarget.transform.localScale = Vector3.one;

            // Head chop

            var headChop = mouthTarget.GetComponent<VRCHeadChop>();
            if (!headChop)
            {
                throw new Exception("VRCHeadChop component not found on mouth target.");
            }

            headChop.globalScaleFactor = 1f;
            if (headChop.targetBones == null || headChop.targetBones.Length != 1)
            {
                headChop.targetBones = new[] { new VRCHeadChop.HeadChopBone() };
            }

            var hcTargetBone = headChop.targetBones[0];
            hcTargetBone.transform = mouthTarget;
            hcTargetBone.scaleFactor = 1f;
            hcTargetBone.applyCondition = VRCHeadChop.HeadChopBone.ApplyCondition.AlwaysApply;

            SetupGizmos(mouthTarget, handColliderRadius, handColliderHeight);
        }

        private void Build()
        {
            var avatar = GetAvatar();
            if (!avatar) throw new Exception("No avatar found");

            // Hand collider.

            var isRightHand = hand == Hand.Right;

            var targetCollider = isRightHand ? avatar.collider_handR : avatar.collider_handL;
            var otherCollider = !isRightHand ? avatar.collider_handR : avatar.collider_handL;

            targetCollider.isMirrored = false;
            targetCollider.position = Vector3.zero;
            targetCollider.rotation = Quaternion.identity;
            targetCollider.transform = handProxy;
            targetCollider.state = VRCAvatarDescriptor.ColliderConfig.State.Custom;

            otherCollider.isMirrored = false;

            if (isRightHand)
            {
                avatar.collider_handR = targetCollider;
                avatar.collider_handL = otherCollider;
            }
            else
            {
                avatar.collider_handL = targetCollider;
                avatar.collider_handR = otherCollider;
            }

            EditorUtility.SetDirty(avatar);
        }

        private static void Log(LogType logType, string s)
        {
            Debug.unityLogger.Log(logType, $"[{nameof(DreamBite)}] {s}");
        }

        public class OnAvatarBuildUpdateSettings : IVRCSDKPreprocessAvatarCallback
        {
            public int callbackOrder => int.MinValue;

            public bool OnPreprocessAvatar(GameObject avatarGameObject)
            {
                Log(LogType.Log, $"OnPreprocessAvatar: UpdateSettings: {avatarGameObject.name}");

                var components = avatarGameObject.GetComponentsInChildren<DreamBite>();
                foreach (var component in components)
                {
                    component.UpdateSettings();
                }

                return true;
            }
        }

        public class OnAvatarBuild : IVRCSDKPreprocessAvatarCallback
        {
            public int callbackOrder => 0;

            public bool OnPreprocessAvatar(GameObject avatarGameObject)
            {
                Log(LogType.Log, $"OnPreprocessAvatar: Build: {avatarGameObject.name}");

                var components = avatarGameObject.GetComponentsInChildren<DreamBite>();
                foreach (var component in components)
                {
                    component.Build();
                }

                return true;
            }
        }

        [CustomEditor(typeof(DreamBite))]
        private class DreamBiteEditor : Editor
        {
            private SerializedProperty _propertyHand;
            private SerializedProperty _propertyFixHandRotationMode;
            private SerializedProperty _propertyFixHandRotation;
            private SerializedProperty _propertyGizmoColor;
            private SerializedProperty _propertyHandProxy;
            private SerializedProperty _propertyMouthTarget;

            private void OnEnable()
            {
                _propertyHand = serializedObject.FindProperty(nameof(hand));
                _propertyFixHandRotationMode = serializedObject.FindProperty(nameof(fixHandRotationMode));
                _propertyFixHandRotation = serializedObject.FindProperty(nameof(fixHandRotation));
                _propertyGizmoColor = serializedObject.FindProperty(nameof(gizmoColor));
                _propertyHandProxy = serializedObject.FindProperty(nameof(handProxy));
                _propertyMouthTarget = serializedObject.FindProperty(nameof(mouthTarget));

                var dreamBite = target as DreamBite;
                if (!dreamBite) return;
                dreamBite.UpdateSettings();
            }

            private void OnDisable()
            {
                var dreamBite = target as DreamBite;
                if (!dreamBite) return;
                dreamBite.UpdateSettings();
            }

            public override void OnInspectorGUI()
            {
                var dreamBite = target as DreamBite;
                if (!dreamBite)
                {
                    EditorGUILayout.HelpBox("DreamBite component not found", MessageType.Error);
                    return;
                }

                if (!dreamBite.GetAvatar())
                {
                    EditorGUILayout.HelpBox("Avatar not found, move Prefab to an avatar GameObject", MessageType.Error);
                }

                serializedObject.Update();

                var needUpdate = false;
                var needRepaint = false;

                EditorGUI.BeginChangeCheck();
                GUILayout.BeginHorizontal();
                GUILayout.Label("Hand Collider");
                _propertyHand.enumValueIndex =
                    GUILayout.Toolbar(_propertyHand.enumValueIndex, new[] { "Left", "Right" });
                GUILayout.EndHorizontal();

                GUILayout.BeginHorizontal();
                GUILayout.Label("Fix Hand Rotation");

                _propertyFixHandRotationMode.enumValueIndex =
                    GUILayout.Toolbar(_propertyFixHandRotationMode.enumValueIndex, new[] { "None", "Auto", "Custom" });
                GUILayout.EndHorizontal();

                if (_propertyFixHandRotationMode.enumValueIndex == (int)FixHandRotationMode.Custom)
                {
                    EditorGUILayout.PropertyField(_propertyFixHandRotation);
                }


                EditorGUILayout.PropertyField(_propertyGizmoColor);
                if (EditorGUI.EndChangeCheck())
                {
                    needUpdate = true;
                }

                GUILayout.Space(10);

                var handProxyConstraint = dreamBite.handProxy.GetComponent<VRCParentConstraint>();
                if (handProxyConstraint)
                {
                    GUILayout.Label("Move me for testing:");
                    var newWeight = EditorGUILayout.Slider(handProxyConstraint.GlobalWeight, 0, 1);
                    if (Mathf.Abs(handProxyConstraint.GlobalWeight - newWeight) > 0.001f)
                    {
                        handProxyConstraint.GlobalWeight = newWeight;
                        needRepaint = true;
                    }

                    GUILayout.Space(10);
                }

                EditorGUILayout.PropertyField(_propertyHandProxy);
                EditorGUILayout.PropertyField(_propertyMouthTarget);

                serializedObject.ApplyModifiedProperties();

                if (needUpdate)
                {
                    dreamBite.UpdateSettings();
                }

                if (needRepaint || needUpdate)
                {
                    Repaint();
                    SceneView.RepaintAll();
                }
            }
        }
    }
}

#endif